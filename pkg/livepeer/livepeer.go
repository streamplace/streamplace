package livepeer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
)

const SegmentsInFlight = 2

// MaxWaiting is how many segments may queue for a transcode slot before
// further ones are skipped (ErrBacklog). A transcoder that can't keep up
// with real time otherwise builds an ever-growing queue: renditions fall
// further behind live and the node holds every waiting segment in memory.
// Skipping keeps the renditions near live at the cost of a gap in them.
const MaxWaiting = 2

// ErrNoKeyframe is returned for a segment whose video has no IDR with its
// parameter sets: a transcoder decodes each pushed segment from scratch, so
// it cannot decode such a segment and, worse, a go-livepeer gateway drops
// its orchestrator session over the failure. The segment is skipped (a
// gap in the renditions) rather than pushed. The ingest path keeps such
// frames inside their GoP (media.installIDRKeyframeProbe); this guards the
// transcoder from any other source of them.
var ErrNoKeyframe = errors.New("segment does not start with an IDR and its parameter sets")

// tsStartsDecodable reports whether an MPEG-TS video segment carries SPS,
// PPS and an IDR slice — what a fresh decoder needs.
func tsStartsDecodable(ts []byte) bool {
	var sps, pps, idr bool
	for i := 0; i+3 < len(ts); {
		if ts[i] == 0 && ts[i+1] == 0 && ts[i+2] == 1 {
			switch ts[i+3] & 31 {
			case 7:
				sps = true
			case 8:
				pps = true
			case 5:
				idr = true
			}
			if sps && pps && idr {
				return true
			}
			i += 4
			continue
		}
		i++
	}
	return false
}

// ErrBacklog is returned for a segment skipped because too many are already
// waiting for a transcode slot.
var ErrBacklog = errors.New("transcode backlog: segment skipped")

// RotateAfterFailures is how many pushes in a row the gateway may refuse
// before the session moves to a fresh manifest ID. A go-livepeer gateway
// keeps per-manifest state (its orchestrator sessions and suspensions) for
// as long as segments keep arriving under that manifest, so a manifest that
// has fallen into "No sessions available" stays there forever while a fresh
// one on the same gateway transcodes fine: that is exactly what a stuck
// production stream looked like. A new manifest is a new stream to the
// gateway; the segment sequence restarts at 0 with it.
const RotateAfterFailures = 5

type LivepeerSession struct {
	// SessionID is the manifest ID's base: "<did>-<trailer>", a new
	// trailer per rotation. Guarded by mu, like Count.
	SessionID  string
	Count      int
	GatewayURL string
	Guard      chan struct{}
	CLI        *config.CLI
	waiting    atomic.Int32
	mu         sync.Mutex
	did        string
	failures   int
}

// borrowed from catalyst-api
func RandomTrailer(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	res := make([]byte, length)
	for i := 0; i < length; i++ {
		res[i] = charset[rand.Intn(len(charset))]
	}
	return string(res)
}

func NewLivepeerSession(ctx context.Context, cli *config.CLI, did string, gatewayURL string) (*LivepeerSession, error) {
	return &LivepeerSession{
		SessionID:  manifestBase(did),
		Count:      0,
		GatewayURL: gatewayURL,
		Guard:      make(chan struct{}, SegmentsInFlight),
		CLI:        cli,
		did:        did,
	}, nil
}

func manifestBase(did string) string {
	sessionID := fmt.Sprintf("%s-%s", did, RandomTrailer(8))
	sessionID = strings.ReplaceAll(sessionID, ":", "")
	return strings.ReplaceAll(sessionID, ".", "")
}

// next claims the manifest ID and sequence number for one push.
func (ls *LivepeerSession) next(numRenditions int) (string, int) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	seq := ls.Count
	ls.Count++
	return fmt.Sprintf("%s-%dren", ls.SessionID, numRenditions), seq
}

// noteResult records whether the gateway took a push; RotateAfterFailures
// refusals in a row move the session to a fresh manifest.
func (ls *LivepeerSession) noteResult(ctx context.Context, ok bool) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ok {
		ls.failures = 0
		return
	}
	ls.failures++
	if ls.failures < RotateAfterFailures {
		return
	}
	old := ls.SessionID
	ls.SessionID = manifestBase(ls.did)
	ls.Count = 0
	ls.failures = 0
	spmetrics.TranscodeManifestRotationsTotal.Inc()
	log.Warn(ctx, "transcode: gateway refused every recent push, moving to a fresh manifest", "failures", RotateAfterFailures, "old", old, "new", ls.SessionID)
}

func (ls *LivepeerSession) PostSegmentToGateway(ctx context.Context, buf []byte, spseg *placestream.Segment, rs renditions.Renditions) ([][]byte, error) {
	ctx = log.WithLogValues(ctx, "func", "PostSegmentToGateway")
	lpProfiles := rs.ToLivepeerProfiles()
	tsSeg := bytes.Buffer{}
	audioSeg := bytes.Buffer{}
	// Bounded: a conversion pipeline that never reaches EOS must not hold
	// the in-flight guard forever and silently stop every later transcode.
	convCtx, convCancel := context.WithTimeout(ctx, 30*time.Second)
	err := media.MP4ToMPEGTSVideoMP4Audio(convCtx, bytes.NewReader(buf), &tsSeg, &audioSeg)
	convCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to convert mp4 to ts video/mp4 audio: %w", err)
	}
	if tsSeg.Len() == 0 {
		return nil, fmt.Errorf("no video in segment")
	}
	if audioSeg.Len() == 0 {
		return nil, fmt.Errorf("no audio in segment")
	}
	if !tsStartsDecodable(tsSeg.Bytes()) {
		return nil, ErrNoKeyframe
	}
	if ls.waiting.Add(1) > MaxWaiting {
		ls.waiting.Add(-1)
		return nil, ErrBacklog
	}
	ls.Guard <- struct{}{}
	ls.waiting.Add(-1)
	start := time.Now()
	// check if context is done since we were waiting for the lock
	if ctx.Err() != nil {
		<-ls.Guard
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()
	// Claimed only now, with the slot: the manifest may have rotated while
	// this segment waited.
	sessionIDRen, seqNo := ls.next(len(rs))
	transcodingConfiguration := map[string]any{
		"manifestID": sessionIDRen,
		"profiles":   lpProfiles,
	}
	bs, err := json.Marshal(transcodingConfiguration)
	if err != nil {
		<-ls.Guard
		return nil, fmt.Errorf("failed to marshal livepeer profile: %w", err)
	}
	url := fmt.Sprintf("%s/live/%s/%d.ts", ls.GatewayURL, sessionIDRen, seqNo)

	dur := time.Duration(*spseg.Duration)
	durationMs := int(dur.Milliseconds())
	log.Debug(ctx, "posting segment to livepeer gateway", "duration_ms", durationMs, "url", url)

	vid := spseg.Video[0]
	width := int(vid.Width)
	height := int(vid.Height)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(tsSeg.Bytes()))
	if err != nil {
		<-ls.Guard
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Content-Duration", fmt.Sprintf("%d", durationMs))
	req.Header.Set("Content-Resolution", fmt.Sprintf("%dx%d", width, height))
	req.Header.Set("Livepeer-Transcode-Configuration", string(bs))

	if ls.CLI.LivepeerDebug {
		debugDir := ls.CLI.DataFilePath([]string{"livepeer-debug"})
		err = os.MkdirAll(debugDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create debug directory: %w", err)
		}
		debugFile := fmt.Sprintf("%s/livepeer-debug/%s-%06d-input.ts", ls.CLI.DataDir, sessionIDRen, seqNo)
		err = os.WriteFile(debugFile, tsSeg.Bytes(), 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to write debug file: %w", err)
		}
		bs, err := json.MarshalIndent(req.Header, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal livepeer profile: %w", err)
		}
		configFile := fmt.Sprintf("%s/livepeer-debug/%s-%06d-config.json", ls.CLI.DataDir, sessionIDRen, seqNo)
		err = os.WriteFile(configFile, bs, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to write debug file: %w", err)
		}
		log.Log(ctx, "wrote debug file", "file", debugFile)
	}

	resp, err := aqhttp.DoTrusted(ctx, req)
	if err != nil {
		<-ls.Guard
		ls.noteResult(ctx, false)
		return nil, fmt.Errorf("failed to send segment to gateway (config %s): %w", string(bs), err)
	}
	<-ls.Guard
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errOut, _ := io.ReadAll(resp.Body)
		ls.noteResult(ctx, false)
		return nil, fmt.Errorf("gateway returned non-OK status (config %s): %d, %s", string(bs), resp.StatusCode, string(errOut))
	}
	ls.noteResult(ctx, true)

	var out [][]byte

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse media type: %w", err)
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(resp.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to get next part: %w", err)
			}
			ctx := log.WithLogValues(ctx, "part", p.FileName())
			mp4Bs := bytes.Buffer{}
			audioReader := bytes.NewReader(audioSeg.Bytes())
			if ls.CLI.LivepeerDebug {
				debugFile := fmt.Sprintf("%s/livepeer-debug/%s-%06d-output-%s", ls.CLI.DataDir, sessionIDRen, seqNo, p.FileName())
				err = os.WriteFile(debugFile, tsSeg.Bytes(), 0644)
				if err != nil {
					return nil, fmt.Errorf("failed to write debug file: %w", err)
				}
				log.Log(ctx, "wrote debug file", "file", debugFile)
			}
			err = media.MPEGTSVideoMP4AudioToMP4(ctx, p, audioReader, &mp4Bs)
			if err != nil {
				return nil, fmt.Errorf("failed to convert ts to mp4: %w", err)
			}
			bs := mp4Bs.Bytes()
			log.Debug(ctx, "got part back from livepeer gateway", "length", len(bs), "name", p.FileName())
			out = append(out, bs)
		}
	}
	spmetrics.TranscodeDuration.WithLabelValues(spseg.Creator).Observe(float64(time.Since(start).Milliseconds()))
	return out, nil
}
