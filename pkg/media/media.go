package media

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"

	"stream.place/streamplace/pkg/log"

	"github.com/piprate/json-gold/ld"

	irohStreamplace "stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"

	_ "stream.place/streamplace/pkg/streamplacedeps"
)

const CertFile = "cert.pem"
const SegmentsDir = "segments"

const StreamplaceMetadata = "cawg.metadata"

type MediaManager struct {
	cli                 *config.CLI
	liveWindows         map[string]*livehls.Writer
	liveWindowsMut      sync.Mutex
	httpPipes           map[string]io.Writer
	httpPipesMutex      sync.Mutex
	newSegmentSubs      []chan *NewSegmentNotification
	newSegmentSubsMutex sync.RWMutex
	model               model.Model
	bus                 *bus.Bus
	atsync              *atproto.ATProtoSynchronizer
	webrtcAPI           *webrtc.API
	webrtcConfig        webrtc.Configuration
	localDB             localdb.LocalDB

	// Node S2PA transcode signer (cert + PKCS#8 key PEM), built once from the
	// server-repo key. Used to sign transcode-completed audio tracks under the
	// node's own did:web identity, signed in-wasm (the node key is software).
	// See transcode.go.
	nodeSignerOnce sync.Once
	nodeCert       []byte
	nodeKeyPEM     []byte
	nodeSignerErr  error

	// Per-stream continuous audio transcoders, keyed by repoDID. A single-codec
	// stream's segments are fed here in order; each completed dual-codec segment
	// is distributed asynchronously (~1 GoP later). See transcode_stream.go.
	transcoders   map[string]*streamTranscoder
	transcodersMu sync.Mutex

	// Monotonic ingest-session epoch. Each live ingest session (one
	// SegmentAndSignElem) claims a fresh value, stamped onto its context, so the
	// per-DID transcoder rebuilds when a streamer reconnects rather than feeding
	// the restarted media timeline into the previous session's continuous encoder.
	// See withIngestSession / feedStreamTranscoder.
	ingestSessionSeq atomic.Uint64
}

// nextIngestSession claims a fresh monotonic ingest-session epoch for a new live
// session. Epochs are strictly increasing, so a newer session always wins over a
// transcoder built for an older one.
func (mm *MediaManager) nextIngestSession() uint64 {
	return mm.ingestSessionSeq.Add(1)
}

type NewSegmentNotification struct {
	Segment *localdb.Segment
	// Data is the presentation flat MP4 (ftyp+moov+mdat envelope) consumed by
	// the GStreamer pipelines (WebRTC packetize, thumbnail).
	Data []byte
	// Muxl is the bare canonical .m4s: blindly concatenatable signed segments
	// with no container header. Consumers synthesize whatever wrapper they
	// need. The long-term wire format; Data retires once all consumers are MUXL.
	Muxl     []byte
	Metadata *SegmentMetadata
	Local    bool
}

func RunSelfTest(ctx context.Context) error {
	gstinit.InitGST()
	return SelfTest(ctx)
}

func MakeMediaManager(ctx context.Context, cli *config.CLI, signer crypto.Signer, mod model.Model, bus *bus.Bus, atsync *atproto.ATProtoSynchronizer, ldb localdb.LocalDB) (*MediaManager, error) {
	gstinit.InitGST()
	err := SelfTest(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in gstreamer self-test: %w", err)
	}

	api, config, err := newWebRTCAPI()
	if err != nil {
		return nil, err
	}
	return &MediaManager{
		cli:          cli,
		liveWindows:  map[string]*livehls.Writer{},
		httpPipes:    map[string]io.Writer{},
		model:        mod,
		bus:          bus,
		atsync:       atsync,
		webrtcAPI:    api,
		webrtcConfig: config,
		localDB:      ldb,
		transcoders:  map[string]*streamTranscoder{},
	}, nil
}

func (mm *MediaManager) HandleData(node *irohStreamplace.PublicKey, data []byte) {
	r := bytes.NewReader(data)
	ctx := context.Background()
	err := mm.ValidateMP4(ctx, r, true)
	if err != nil {
		log.Log(ctx, "invalid incoming segment", "error", err)
	}
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
	StartTime             aqtime.AQTime
	Title                 string
	Creator               string
	ContentWarnings       []string
	ContentRights         *localdb.ContentRights
	DistributionPolicy    *localdb.DistributionPolicy
	MetadataConfiguration *streamplace.MetadataConfiguration
	Livestream            *streamplace.Livestream
	Published             bool
}

var ErrMissingMetadata = errors.New("missing segment metadata")
var ErrInvalidMetadata = errors.New("invalid segment metadata")
var C2PAActionsV2Label = "c2pa.actions.v2"
var C2PAPublishedAction = "c2pa.published"

func ParseSegmentAssertions(ctx context.Context, mani *c2patypes.Manifest) (*SegmentMetadata, error) {
	_, span := otel.Tracer("signer").Start(ctx, "ParseSegmentAssertions")
	defer span.End()
	var ass *c2patypes.ManifestAssertion
	isPublished := false
	for _, a := range mani.Assertions {
		if a.Label == StreamplaceMetadata {
			ass = &a
			continue
		}
		if a.Label == "place.stream.metadata" {
			// backwards compatibility for old manifests
			ass = &a
			continue
		}
		if a.Label == C2PAActionsV2Label {
			data, ok := a.Data.(map[string]any)
			if !ok {
				return nil, ErrInvalidMetadata
			}
			actions, ok := data["actions"].([]any)
			if !ok {
				return nil, ErrInvalidMetadata
			}
			for _, action := range actions {
				actionMap, ok := action.(map[string]any)
				if !ok {
					return nil, ErrInvalidMetadata
				}
				actionType, ok := actionMap["action"].(string)
				if !ok {
					return nil, ErrInvalidMetadata
				}
				if actionType == C2PAPublishedAction {
					isPublished = true
					break
				}
			}
		}
	}
	if ass == nil {
		return nil, ErrMissingMetadata
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

	contentWarnings := extractContentWarnings(mani)
	contentRights := extractContentRights(mani)
	distributionPolicy := extractDistributionPolicy(mani, start)
	metadataConfiguration := extractMetadataConfiguration(mani)
	livestream := extractLivestream(mani)

	out := SegmentMetadata{
		StartTime:             start,
		Title:                 meta.Title[0].Value,
		Creator:               meta.Creator[0].Value,
		ContentWarnings:       contentWarnings,
		ContentRights:         contentRights,
		DistributionPolicy:    distributionPolicy,
		MetadataConfiguration: metadataConfiguration,
		Livestream:            livestream,
		Published:             isPublished,
	}
	return &out, nil
}

// findAssertion finds an assertion by label
func findAssertion(mani *c2patypes.Manifest, label string) *c2patypes.ManifestAssertion {
	for _, a := range mani.Assertions {
		if a.Label == label {
			return &a
		}
	}
	return nil
}

// extractContentWarnings extracts content warnings from the C2PA manifest
func extractContentWarnings(mani *c2patypes.Manifest) []string {
	ass := findAssertion(mani, StreamplaceMetadata)
	if ass == nil {
		return nil
	}

	data, ok := ass.Data.(map[string]interface{})
	if !ok {
		return nil
	}

	warnings, ok := data["Iptc4xmpExt:ContentWarning"]
	if !ok {
		return nil
	}

	warningList, ok := warnings.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(warningList))
	for _, warning := range warningList {
		if warningStr, ok := warning.(string); ok {
			result = append(result, warningStr)
		}
	}

	return result
}

// extractContentRights extracts content rights from the C2PA manifest
func extractContentRights(mani *c2patypes.Manifest) *localdb.ContentRights {
	ass := findAssertion(mani, StreamplaceMetadata)
	if ass == nil {
		return nil
	}

	data, ok := ass.Data.(map[string]interface{})
	if !ok {
		return nil
	}

	rights := &localdb.ContentRights{}

	// Extract copyright notice
	if notice, ok := data["dc:rights"]; ok {
		if noticeStr, ok := notice.(string); ok {
			rights.CopyrightNotice = &noticeStr
		}
	}

	// Extract copyright year
	if year, ok := data["Iptc4xmpExt:CopyrightYear"]; ok {
		if yearNum, ok := year.(float64); ok {
			yearInt := int64(yearNum)
			rights.CopyrightYear = &yearInt
		}
	}

	// Extract creator
	if creator, ok := data["dc:creator"]; ok {
		if creatorStr, ok := creator.(string); ok {
			rights.Creator = &creatorStr
		}
	}

	// Extract credit line
	if credit, ok := data["photoshop:Credit"]; ok {
		if creditStr, ok := credit.(string); ok {
			rights.CreditLine = &creditStr
		}
	}

	// Extract license information
	if license, ok := data["Iptc4xmpExt:LinkedEncRightsExpr"]; ok {
		if licenseStr, ok := license.(string); ok {
			rights.License = &licenseStr
		}
	} else if usageTerms, ok := data["xmpRights:UsageTerms"]; ok {
		if usageStr, ok := usageTerms.(string); ok {
			rights.License = &usageStr
		}
	}

	// Return nil if no rights information was found
	if rights.CopyrightNotice == nil && rights.CopyrightYear == nil &&
		rights.Creator == nil && rights.CreditLine == nil && rights.License == nil {
		return nil
	}

	return rights
}

// extractDistributionPolicy extracts distribution policy from the C2PA manifest
func extractDistributionPolicy(mani *c2patypes.Manifest, segmentStart aqtime.AQTime) *localdb.DistributionPolicy {
	metadataConfig := extractMetadataConfiguration(mani)
	if metadataConfig == nil {
		return nil
	}

	if metadataConfig.DistributionPolicy == nil {
		return nil
	}

	if metadataConfig.DistributionPolicy.DeleteAfter == nil {
		return nil
	}

	// deleteAfter contains an offset in seconds from creation time
	deleteAfterSeconds := *metadataConfig.DistributionPolicy.DeleteAfter

	return &localdb.DistributionPolicy{
		DeleteAfterSeconds: &deleteAfterSeconds,
	}
}

// extractMetadataConfiguration extracts the place.stream.metadata.configuration from the C2PA manifest
func extractMetadataConfiguration(mani *c2patypes.Manifest) *streamplace.MetadataConfiguration {
	ass := findAssertion(mani, "place.stream.metadata.configuration")
	if ass == nil {
		return nil
	}

	bs, err := json.Marshal(ass.Data)
	if err != nil {
		return nil
	}
	var metadataConfiguration streamplace.MetadataConfiguration
	err = json.Unmarshal(bs, &metadataConfiguration)
	if err != nil {
		return nil
	}
	return &metadataConfiguration
}

func extractLivestream(mani *c2patypes.Manifest) *streamplace.Livestream {
	ass := findAssertion(mani, "place.stream.livestream")
	if ass == nil {
		return nil
	}
	bs, err := json.Marshal(ass.Data)
	if err != nil {
		return nil
	}

	var livestream streamplace.Livestream
	err = json.Unmarshal(bs, &livestream)
	if err != nil {
		return nil
	}
	return &livestream
}
