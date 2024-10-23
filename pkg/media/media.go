package media

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aquareum.tv/aquareum/pkg/aqtime"
	"aquareum.tv/aquareum/pkg/config"
	"aquareum.tv/aquareum/pkg/crypto/signers"
	"aquareum.tv/aquareum/pkg/log"
	"aquareum.tv/aquareum/pkg/model"
	"aquareum.tv/aquareum/pkg/replication"
	"github.com/go-gst/go-gst/gst"
	"github.com/google/uuid"
	"github.com/livepeer/lpms/ffmpeg"
	"golang.org/x/sync/errgroup"

	"git.aquareum.tv/aquareum-tv/c2pa-go/pkg/c2pa"
	"git.aquareum.tv/aquareum-tv/c2pa-go/pkg/c2pa/generated/manifeststore"
	"github.com/piprate/json-gold/ld"
)

const CERT_FILE = "cert.pem"
const SEGMENTS_DIR = "segments"
const STDS_METADATA = "stds.metadata"
const SCHEMA_ORG_VIDEO_OBJECT = "http://schema.org/VideoObject"
const SCHEMA_ORG_START_TIME = "http://schema.org/startTime"
const SCHEMA_ORG_END_TIME = "http://schema.org/endTime"

type MediaManager struct {
	cli                 *config.CLI
	mp4subs             map[string][]chan string
	mp4subsmut          sync.Mutex
	replicator          replication.Replicator
	hlsRunning          map[string]HLSStream
	hlsRunningMut       sync.Mutex
	httpPipes           map[string]io.Writer
	httpPipesMutex      sync.Mutex
	newSegmentSubs      []chan *NewSegmentNotification
	newSegmentSubsMutex sync.RWMutex
}

type NewSegmentNotification struct {
	Segment *model.Segment
	Data    []byte
}

type HLSStream struct {
	Dir  string
	Wait func() string
}

func RunSelfTest(ctx context.Context) error {
	gst.Init(nil)
	return SelfTest(ctx)
}

func MakeMediaManager(ctx context.Context, cli *config.CLI, signer crypto.Signer, rep replication.Replicator) (*MediaManager, error) {
	gst.Init(nil)
	err := SelfTest(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in gstreamer self-test: %w", err)
	}
	return &MediaManager{
		cli:        cli,
		mp4subs:    map[string][]chan string{},
		replicator: rep,
		hlsRunning: map[string]HLSStream{},
		httpPipes:  map[string]io.Writer{},
	}, nil
}

// replacement for os.Pipe that works on windows
func (mm *MediaManager) HTTPPipe() (string, io.Reader, func(), error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", nil, nil, err
	}
	mm.httpPipesMutex.Lock()
	defer mm.httpPipesMutex.Unlock()
	u := fmt.Sprintf("%s/http-pipe/%s", mm.cli.OwnInternalURL(), uu.String())
	done := func() {
		mm.httpPipesMutex.Lock()
		defer mm.httpPipesMutex.Unlock()
		delete(mm.httpPipes, uu.String())
	}
	r, w := io.Pipe()
	mm.httpPipes[uu.String()] = w
	return u, r, done, nil
}

func (mm *MediaManager) GetHTTPPipeWriter(uu string) io.Writer {
	mm.httpPipesMutex.Lock()
	defer mm.httpPipesMutex.Unlock()
	return mm.httpPipes[uu]
}

// register a handler for all new segments that come in
func (mm *MediaManager) NewSegment() <-chan *NewSegmentNotification {
	ch := make(chan *NewSegmentNotification)
	mm.newSegmentSubsMutex.Lock()
	defer mm.newSegmentSubsMutex.Unlock()
	mm.newSegmentSubs = append(mm.newSegmentSubs, ch)
	return ch
}

// subscribe to the latest segments from a given user for livestreaming purposes
func (mm *MediaManager) SubscribeSegment(ctx context.Context, user string) chan string {
	mm.mp4subsmut.Lock()
	defer mm.mp4subsmut.Unlock()
	_, ok := mm.mp4subs[user]
	if !ok {
		mm.mp4subs[user] = []chan string{}
	}
	c := make(chan string)
	mm.mp4subs[user] = append(mm.mp4subs[user], c)
	return c
}

// subscribe to the latest segments from a given user for livestreaming purposes
func (mm *MediaManager) PublishSegment(ctx context.Context, user, file string) {
	mm.mp4subsmut.Lock()
	defer mm.mp4subsmut.Unlock()
	for _, sub := range mm.mp4subs[user] {
		sub <- file
	}
	mm.mp4subs[user] = []chan string{}
}

func (mm *MediaManager) SegmentToMKVPlusOpus(ctx context.Context, user string, w io.Writer) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "matroska",
	}
	pr, pw := io.Pipe()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return mm.SegmentToStream(ctx, user, muxer, pw)
	})
	g.Go(func() error {
		return AddOpusToMKV(ctx, pr, w)
	})
	return g.Wait()
}

func (mm *MediaManager) SegmentToHLSOnce(ctx context.Context, user string) (func() string, error) {
	mm.hlsRunningMut.Lock()
	defer mm.hlsRunningMut.Unlock()
	hls, ok := mm.hlsRunning[user]
	if !ok {
		dname, err := os.MkdirTemp("", "aquareum-hls")
		if err != nil {
			return nil, err
		}
		wait := sync.OnceValue[string](func() string {
			fpath := filepath.Join(dname, HLS_PLAYLIST)
			for {
				_, err := os.Stat(fpath)
				if err == nil {
					break
				}
				if !errors.Is(err, os.ErrNotExist) {
					log.Log(ctx, "unexpected error polling for HLS playlist", "error", err)
				}
				time.Sleep(500 * time.Millisecond)
			}
			return dname
		})
		hls = HLSStream{
			Wait: wait,
			Dir:  dname,
		}
		mm.hlsRunning[user] = hls
		go func() {
			err := mm.SegmentToHLS(ctx, user, dname)
			if err != nil {
				log.Log(ctx, "error in async segmentToHLS code", "error", err)
			}
			mm.hlsRunningMut.Lock()
			defer mm.hlsRunningMut.Unlock()
			delete(mm.hlsRunning, user)
		}()
	}
	return hls.Wait, nil
}

func (mm *MediaManager) SegmentToHLS(ctx context.Context, user, dir string) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "matroska",
	}

	pr, pw := io.Pipe()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return mm.SegmentToStream(ctx, user, muxer, pw)
	})
	g.Go(func() error {
		return ToHLS(ctx, pr, dir)
	})
	return g.Wait()
}

func (mm *MediaManager) SegmentToMP4(ctx context.Context, user string, w io.Writer) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "mp4",
		Opts: map[string]string{
			"movflags": "frag_keyframe+empty_moov",
		},
	}
	return mm.SegmentToStream(ctx, user, muxer, w)
}

func (mm *MediaManager) SegmentToStream(ctx context.Context, user string, muxer ffmpeg.ComponentOptions, w io.Writer) error {
	tc := ffmpeg.NewTranscoder()
	defer tc.StopTranscoder()
	ourl, or, odone, err := mm.HTTPPipe()
	if err != nil {
		return err
	}
	defer odone()
	iname := fmt.Sprintf("%s/playback/%s/concat", mm.cli.OwnInternalURL(), user)
	in := &ffmpeg.TranscodeOptionsIn{
		Fname:       iname,
		Transmuxing: true,
		Profile:     ffmpeg.VideoProfile{},
		Loop:        -1,
		Demuxer: ffmpeg.ComponentOptions{
			Name: "concat",
			Opts: map[string]string{
				"safe":               "0",
				"protocol_whitelist": "file,http,https,tcp,tls",
			},
		},
	}
	out := []ffmpeg.TranscodeOptions{
		{
			Oname: ourl,
			VideoEncoder: ffmpeg.ComponentOptions{
				Name: "copy",
			},
			AudioEncoder: ffmpeg.ComponentOptions{
				Name: "copy",
			},
			Profile: ffmpeg.VideoProfile{Format: ffmpeg.FormatNone},
			Muxer:   muxer,
		},
	}
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		_, err := tc.Transcode(in, out)
		tc.StopTranscoder()
		return err
	})
	g.Go(func() error {
		_, err := io.Copy(w, or)
		return err
	})
	return g.Wait()
}

type obj map[string]any

type StringVal struct {
	Value string `json:"@value"`
}

type ExpandedSchemaOrg []struct {
	Type    []string `json:"@type"`
	Creator []struct {
		Type    []string    `json:"@type"`
		Address []StringVal `json:"http://schema.org/address"`
		Name    []StringVal `json:"http://schema.org/name"`
	} `json:"http://schema.org/creator"`
	StartTime []StringVal `json:"http://schema.org/startTime"`
	EndTime   []StringVal `json:"http://schema.org/endTime"`
}

type SegmentMetadata struct {
	StartTime aqtime.AQTime
	EndTime   aqtime.AQTime
}

var ErrInvalidMetadata = errors.New("invalid Schema.org Metadata")

func ParseSegmentAssertions(mani *manifeststore.Manifest) (*SegmentMetadata, error) {
	var ass *manifeststore.ManifestAssertion
	for _, a := range mani.Assertions {
		if a.Label == STDS_METADATA {
			ass = &a
			break
		}
	}
	if ass == nil {
		return nil, fmt.Errorf("couldn't find %s assertions", STDS_METADATA)
	}
	proc := ld.NewJsonLdProcessor()
	options := ld.NewJsonLdOptions("")
	flat, err := proc.Expand(ass.Data, options)
	if err != nil {
		return nil, err
	}
	bs, err := json.Marshal(flat)
	if err != nil {
		return nil, err
	}
	var metas ExpandedSchemaOrg
	err = json.Unmarshal(bs, &metas)
	if err != nil {
		return nil, err
	}
	if len(metas) != 1 {
		return nil, ErrInvalidMetadata
	}
	meta := metas[0]
	if len(meta.Type) != 1 {
		return nil, ErrInvalidMetadata
	}
	if meta.Type[0] != SCHEMA_ORG_VIDEO_OBJECT {
		return nil, ErrInvalidMetadata
	}
	if len(meta.StartTime) != 1 {
		return nil, ErrInvalidMetadata
	}
	if len(meta.EndTime) != 1 {
		return nil, ErrInvalidMetadata
	}
	start, err := aqtime.FromString(meta.StartTime[0].Value)
	if err != nil {
		return nil, err
	}
	end, err := aqtime.FromString(meta.EndTime[0].Value)
	if err != nil {
		return nil, err
	}
	out := SegmentMetadata{
		StartTime: start,
		EndTime:   end,
	}
	return &out, nil
}

func (mm *MediaManager) ValidateMP4(ctx context.Context, input io.Reader) error {
	buf, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	r := bytes.NewReader(buf)
	reader, err := c2pa.FromStream(r, "video/mp4")
	if err != nil {
		return err
	}
	mani := reader.GetActiveManifest()
	certs := reader.GetProvenanceCertChain()
	pub, err := signers.ParseES256KCert([]byte(certs))
	if err != nil {
		return err
	}
	found := false
	for _, a := range mm.cli.AllowedStreams {
		if a.Equals(pub) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("got valid segment, but address is not allowed: %s", pub.String())
	}
	meta, err := ParseSegmentAssertions(mani)
	if err != nil {
		return err
	}
	fd, err := mm.cli.SegmentFileCreate(pub.String(), meta.StartTime, "mp4")
	if err != nil {
		return err
	}
	defer fd.Close()
	go mm.replicator.NewSegment(ctx, buf)
	r = bytes.NewReader(buf)
	io.Copy(fd, r)
	base := filepath.Base(fd.Name())
	go mm.PublishSegment(ctx, pub.String(), base)
	seg := &model.Segment{
		ID:        *mani.Label,
		User:      pub.String(),
		StartTime: meta.StartTime.Time(),
		EndTime:   meta.EndTime.Time(),
	}
	mm.newSegmentSubsMutex.RLock()
	defer mm.newSegmentSubsMutex.RUnlock()
	not := &NewSegmentNotification{
		Segment: seg,
		Data:    buf,
	}
	for _, ch := range mm.newSegmentSubs {
		go func() { ch <- not }()
	}
	log.Log(ctx, "successfully ingested segment", "user", pub.String(), "timestamp", meta.StartTime)
	return nil
}
