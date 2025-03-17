package media

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/go-gst/go-gst/gst"
	"github.com/google/uuid"
	"github.com/livepeer/lpms/ffmpeg"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"

	"stream.place/streamplace/pkg/replication"

	"git.stream.place/streamplace/c2pa-go/pkg/c2pa"
	"git.stream.place/streamplace/c2pa-go/pkg/c2pa/generated/manifeststore"
	"github.com/piprate/json-gold/ld"
)

const CERT_FILE = "cert.pem"
const SEGMENTS_DIR = "segments"

var STREAMPLACE_METADATA = "place.stream.metadata"

type MediaManager struct {
	cli                 *config.CLI
	mp4subs             map[string][]chan string
	mp4subsmut          sync.Mutex
	replicator          replication.Replicator
	hlsRunning          map[string]*M3U8
	hlsRunningMut       sync.Mutex
	httpPipes           map[string]io.Writer
	httpPipesMutex      sync.Mutex
	newSegmentSubs      []chan *NewSegmentNotification
	newSegmentSubsMutex sync.RWMutex
	model               model.Model
	bus                 *bus.Bus
	atsync              *atproto.ATProtoSynchronizer
}

type NewSegmentNotification struct {
	Segment  *model.Segment
	Data     []byte
	Metadata *SegmentMetadata
}

func RunSelfTest(ctx context.Context) error {
	gst.Init(&[]string{})
	return SelfTest(ctx)
}

func MakeMediaManager(ctx context.Context, cli *config.CLI, signer crypto.Signer, rep replication.Replicator, mod model.Model, bus *bus.Bus, atsync *atproto.ATProtoSynchronizer) (*MediaManager, error) {
	gst.Init(nil)
	err := SelfTest(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in gstreamer self-test: %w", err)
	}
	return &MediaManager{
		cli:        cli,
		mp4subs:    map[string][]chan string{},
		replicator: rep,
		hlsRunning: map[string]*M3U8{},
		httpPipes:  map[string]io.Writer{},
		model:      mod,
		bus:        bus,
		atsync:     atsync,
	}, nil
}

// replacement for os.Pipe that works on windows
func (mm *MediaManager) HTTPPipe() (string, io.ReadCloser, func(), error) {
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
func (mm *MediaManager) SubscribeSegment(ctx context.Context, user string) <-chan string {
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

func (mm *MediaManager) UnsubscribeSegment(ctx context.Context, user string, ch <-chan string) {
	mm.mp4subsmut.Lock()
	defer mm.mp4subsmut.Unlock()
	for i, c := range mm.mp4subs[user] {
		if c == ch {
			mm.mp4subs[user] = append(mm.mp4subs[user][:i], mm.mp4subs[user][i+1:]...)
			break
		}
	}
}

// subscribe to the latest segments from a given user for livestreaming purposes
func (mm *MediaManager) PublishSegment(ctx context.Context, user, file string) {
	mm.mp4subsmut.Lock()
	defer mm.mp4subsmut.Unlock()
	for _, sub := range mm.mp4subs[user] {
		go func() {
			sub <- file
		}()
	}
	mm.mp4subs[user] = []chan string{}
}

func (mm *MediaManager) SegmentToMKV(ctx context.Context, user string, w io.Writer) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "matroska",
	}
	return mm.SegmentToStream(ctx, user, muxer, w)
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

func (mm *MediaManager) SegmentToHLSOnce(ctx context.Context, user string) (*M3U8, error) {
	mm.hlsRunningMut.Lock()
	defer mm.hlsRunningMut.Unlock()
	hls, ok := mm.hlsRunning[user]
	if !ok {
		hls = NewM3U8()
		mm.hlsRunning[user] = hls
		go func() {
			err := mm.SegmentToHLS(ctx, user, hls)
			if err != nil {
				log.Log(ctx, "error in async segmentToHLS code", "error", err)
			}
			mm.hlsRunningMut.Lock()
			defer mm.hlsRunningMut.Unlock()
			delete(mm.hlsRunning, user)
		}()
	}
	return hls, nil
}

func (mm *MediaManager) SegmentToHLS(ctx context.Context, user string, m3u8 *M3U8) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "matroska",
	}

	pr, pw := io.Pipe()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return mm.SegmentToStream(ctx, user, muxer, pw)
	})
	g.Go(func() error {
		return mm.ToHLS(ctx, pr, m3u8)
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
		<-ctx.Done()
		or.Close()
		return nil
	})
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
	Creator []StringVal `json:"http://purl.org/dc/elements/1.1/creator"`
	Date    []StringVal `json:"http://purl.org/dc/elements/1.1/date"`
	Title   []StringVal `json:"http://purl.org/dc/elements/1.1/title"`
}

type SegmentMetadata struct {
	StartTime aqtime.AQTime
	Title     string
	Creator   string
}

var ErrInvalidMetadata = errors.New("invalid segment metadata")

func ParseSegmentAssertions(mani *manifeststore.Manifest) (*SegmentMetadata, error) {
	var ass *manifeststore.ManifestAssertion
	for _, a := range mani.Assertions {
		if a.Label == STREAMPLACE_METADATA {
			ass = &a
			break
		}
	}
	if ass == nil {
		return nil, fmt.Errorf("couldn't find %s assertions", STREAMPLACE_METADATA)
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
	if len(meta.Creator) == 0 {
		return nil, ErrInvalidMetadata
	}
	if len(meta.Title) != 1 {
		return nil, ErrInvalidMetadata
	}
	if len(meta.Date) != 1 {
		return nil, ErrInvalidMetadata
	}
	start, err := aqtime.FromString(meta.Date[0].Value)
	if err != nil {
		return nil, err
	}
	out := SegmentMetadata{
		StartTime: start,
		Title:     meta.Title[0].Value,
		Creator:   meta.Creator[0].Value,
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
	meta, err := ParseSegmentAssertions(mani)
	if err != nil {
		return err
	}
	mediaData, err := mm.ParseSegmentMediaData(ctx, buf)
	if err != nil {
		return err
	}
	// special case for test signers that are only signed with a key
	var repoDID string
	var signingKeyDID string
	if strings.HasPrefix(meta.Creator, constants.DID_KEY_PREFIX) {
		signingKeyDID = meta.Creator
		repoDID = meta.Creator
	} else {
		repo, err := mm.atsync.SyncBlueskyRepoCached(ctx, meta.Creator, mm.model)
		if err != nil {
			return err
		}
		signingKey, err := mm.model.GetSigningKey(pub.DIDKey(), repo.DID)
		if err != nil {
			return err
		}
		if signingKey == nil {
			return fmt.Errorf("no signing key found for %s", pub.DIDKey())
		}
		repoDID = repo.DID
		signingKeyDID = signingKey.DID
	}

	err = mm.cli.StreamIsAllowed(repoDID)
	if err != nil {
		return fmt.Errorf("got valid segment, but user %s is not allowed: %w", repoDID, err)
	}
	fd, err := mm.cli.SegmentFileCreate(repoDID, meta.StartTime, "mp4")
	if err != nil {
		return err
	}
	defer fd.Close()
	go mm.replicator.NewSegment(ctx, buf)
	r = bytes.NewReader(buf)
	io.Copy(fd, r)
	go mm.PublishSegment(ctx, repoDID, fd.Name())
	seg := &model.Segment{
		ID:            *mani.Label,
		SigningKeyDID: signingKeyDID,
		RepoDID:       repoDID,
		StartTime:     meta.StartTime.Time(),
		Title:         meta.Title,
		MediaData:     mediaData,
	}
	mm.newSegmentSubsMutex.RLock()
	defer mm.newSegmentSubsMutex.RUnlock()
	not := &NewSegmentNotification{
		Segment:  seg,
		Data:     buf,
		Metadata: meta,
	}
	for _, ch := range mm.newSegmentSubs {
		go func() { ch <- not }()
	}
	log.Log(ctx, "successfully ingested segment", "user", repoDID, "signingKey", signingKeyDID, "timestamp", meta.StartTime, "segmentID", *mani.Label)
	return nil
}
