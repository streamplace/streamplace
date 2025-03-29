package media

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/go-gst/go-gst/gst"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/media/segchanman"
	"stream.place/streamplace/pkg/model"

	"stream.place/streamplace/pkg/replication"

	"git.stream.place/streamplace/c2pa-go/pkg/c2pa/generated/manifeststore"
	"github.com/piprate/json-gold/ld"
)

const CERT_FILE = "cert.pem"
const SEGMENTS_DIR = "segments"

var STREAMPLACE_METADATA = "place.stream.metadata"

type MediaManager struct {
	cli                 *config.CLI
	segChanMan          *segchanman.SegChanMan
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
		segChanMan: segchanman.MakeSegChanMan(),
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
func (mm *MediaManager) SubscribeSegment(ctx context.Context, user string, rendition string) <-chan *segchanman.Seg {
	return mm.segChanMan.SubscribeSegment(ctx, user, rendition)
}

func (mm *MediaManager) UnsubscribeSegment(ctx context.Context, user string, rendition string, ch <-chan *segchanman.Seg) {
	mm.segChanMan.UnsubscribeSegment(ctx, user, rendition, ch)
}

// subscribe to the latest segments from a given user for livestreaming purposes
func (mm *MediaManager) PublishSegment(ctx context.Context, user, rendition string, seg *segchanman.Seg) {
	mm.segChanMan.PublishSegment(ctx, user, rendition, seg)
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
