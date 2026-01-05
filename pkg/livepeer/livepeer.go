package livepeer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/streamplace"
)

const SegmentsInFlight = 2

type StreamUrls struct {
	StreamID      string `json:"stream_id"`
	WhipURL       string `json:"whip_url"`
	WhepURL       string `json:"whep_url"`
	RtmpURL       string `json:"rtmp_url"`
	RtmpOutputURL string `json:"rtmp_output_url"`
	UpdateURL     string `json:"update_url"`
	StatusURL     string `json:"status_url"`
	DataURL       string `json:"data_url"`
	StopURL       string `json:"stop_url"`
}

type LivepeerSession struct {
	SessionID    string
	Count        int
	GatewayURL   string
	Guard        chan struct{}
	CLI          *config.CLI
	aiURLsParsed atomic.Bool
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
	sessionID := fmt.Sprintf("%s-%s", did, RandomTrailer(8))
	sessionID = strings.ReplaceAll(sessionID, ":", "")
	sessionID = strings.ReplaceAll(sessionID, ".", "")
	return &LivepeerSession{
		SessionID:  sessionID,
		Count:      0,
		GatewayURL: gatewayURL,
		Guard:      make(chan struct{}, SegmentsInFlight),
		CLI:        cli,
	}, nil
}

func (ls *LivepeerSession) sendSegmentRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Caller must manage ls.Guard and close resp.Body
	resp, err := aqhttp.DoTrusted(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send segment to gateway (config): %w", err)
	}
	return resp, nil
}

// PostAISegmentToGateway sends the segment to the AI processing endpoint and returns stream URLs (from JSON body).
func (ls *LivepeerSession) PostAISegmentToGateway(ctx context.Context, buf []byte, spseg *streamplace.Segment, rs renditions.Renditions) (*StreamUrls, error) {
	ctx = log.WithLogValues(ctx, "func", "PostSegmentToGateway")
	lpProfiles := rs.ToLivepeerProfiles()
	sessionIDRen := fmt.Sprintf("%s-%dren", ls.SessionID, len(rs))
	transcodingConfiguration := map[string]any{
		"manifestID": sessionIDRen,
		"profiles":   lpProfiles,
	}

	vid := spseg.Video[0]
	ingestWidth := int(vid.Width)
	ingestHeight := int(vid.Height)

	if ls.CLI.LivepeerAIProcessing {
		aiJobSettings := map[string]any{
			"enable_video_ingress": ls.CLI.LivepeerAIEnableVideoIngress,
			"enable_video_egress":  ls.CLI.LivepeerAIEnableVideoEgress,
			"enable_data_output":   ls.CLI.LivepeerAIEnableDataOutput,
		}

		aiJobSettingsJSON, err := json.Marshal(aiJobSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal AI job params: %w", err)
		}
		aiJobSettingsStr := string(aiJobSettingsJSON)

		aiJobParams := map[string]any{
			"height": ingestHeight,
			// "prompts": string(promptsJSONString),
			"width": ingestWidth,
		}
		aiJobParamsStr, err := json.Marshal(aiJobParams)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal AI job params: %w", err)
		}

		log.Debug(ctx, "ai job params", "aiJobParams", aiJobParams)

		transcodingConfiguration["aiParams"] = map[string]any{
			"capability":      ls.CLI.LivepeerAICapability,
			"parameters":      string(aiJobSettingsStr),
			"request":         "{}",
			"timeout_seconds": 60,
			"stream_id":       sessionIDRen,
			"params":          string(aiJobParamsStr), //# TODO: add params here
		}

		if ls.CLI.LivepeerAIStreamKey != "" {
			transcodingConfiguration["streamKey"] = ls.CLI.LivepeerAIStreamKey
		}
	}

	log.Debug(ctx, "transcoding configuration", "transcodingConfiguration", transcodingConfiguration)

	bs, err := json.Marshal(transcodingConfiguration)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal livepeer profile: %w", err)
	}

	// Convert MP4 to muxed MPEG-TS (aligns with transcode ingest format expected by /process/segment)
	tsSeg := bytes.Buffer{}
	audioSeg := bytes.Buffer{}
	err = media.MP4ToMPEGTSVideoMP4Audio(ctx, bytes.NewReader(buf), &tsSeg, &audioSeg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert mp4 to ts for ai: %w", err)
	}
	if tsSeg.Len() == 0 {
		return nil, fmt.Errorf("no video in segment for ai")
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	url_ai := fmt.Sprintf("%s/process/segment", ls.GatewayURL)

	dur := time.Duration(*spseg.Duration)
	durationMs := int(dur.Milliseconds())

	req_ai, err := http.NewRequestWithContext(ctx, "POST", url_ai, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("failed to create AI request: %w", err)
	}
	req_ai.Header.Set("Accept", "multipart/mixed")
	// Send MPEG-TS to match transcode ingest path.
	req_ai.Header.Set("Content-Type", "video/MP2T")
	req_ai.Header.Set("Content-Duration", fmt.Sprintf("%d", durationMs))
	req_ai.Header.Set("Content-Resolution", fmt.Sprintf("%dx%d", ingestWidth, ingestHeight))
	req_ai.Header.Set("Livepeer-Transcode-Configuration", string(bs))

	resp_ai, err := ls.sendSegmentRequest(ctx, req_ai)
	if err != nil {
		return nil, err
	}
	defer resp_ai.Body.Close()

	if resp_ai.StatusCode != http.StatusOK {
		errOut, _ := io.ReadAll(resp_ai.Body)
		return nil, fmt.Errorf("gateway (ai) returned non-OK status (config %s): %d, %s", string(bs), resp_ai.StatusCode, string(errOut))
	}

	// Parse AI response body for stream URLs (JSON, first segment)
	parsedFirst := ls.aiURLsParsed.CompareAndSwap(false, true)
	if !parsedFirst {
		log.Debug(ctx, "skipping ai response parse; already parsed first segment")
		return nil, nil
	}

	var streamUrls *StreamUrls
	contentTypeAI := resp_ai.Header.Get("Content-Type")
	log.Debug(ctx, "checking for AI stream URLs in /process/segment response body", "content_type_ai", contentTypeAI)

	if strings.HasPrefix(contentTypeAI, "application/json") {
		bodyBytes, err := io.ReadAll(resp_ai.Body)
		if err == nil {
			log.Debug(ctx, "read ai response body", "length", len(bodyBytes))
			var urls StreamUrls
			if err := json.Unmarshal(bodyBytes, &urls); err == nil {
				streamUrls = &urls
				log.Log(ctx, "✓ GOT AI STREAM URLS FROM /process/segment BODY",
					"data_url", urls.DataURL, "stream_id", urls.StreamID,
					"whip_url", urls.WhipURL, "whep_url", urls.WhepURL,
					"rtmp_url", urls.RtmpURL, "update_url", urls.UpdateURL)
			} else {
				log.Error(ctx, "failed to parse ai response body as json", "error", err, "body", string(bodyBytes))
			}
		} else {
			log.Error(ctx, "failed to read ai response body", "error", err)
		}
	} else {
		log.Debug(ctx, "ai response is not json", "content_type", contentTypeAI)
	}

	if streamUrls == nil {
		log.Log(ctx, "WARNING: No AI stream URLs found in /process/segment response body")
	}
	return streamUrls, nil
}

// PostSegmentToGateway sends the segment to the transcode endpoint and returns renditions.
func (ls *LivepeerSession) PostSegmentToGateway(ctx context.Context, buf []byte, spseg *streamplace.Segment, rs renditions.Renditions) ([][]byte, error) {
	ctx = log.WithLogValues(ctx, "func", "PostSegmentToGateway")
	lpProfiles := rs.ToLivepeerProfiles()
	sessionIDRen := fmt.Sprintf("%s-%dren", ls.SessionID, len(rs))
	transcodingConfiguration := map[string]any{
		"manifestID": sessionIDRen,
		"profiles":   lpProfiles,
	}

	vid := spseg.Video[0]
	ingestWidth := int(vid.Width)
	ingestHeight := int(vid.Height)

	if ls.CLI.LivepeerAIProcessing {
		aiJobSettings := map[string]any{
			"enable_video_ingress": ls.CLI.LivepeerAIEnableVideoIngress,
			"enable_video_egress":  ls.CLI.LivepeerAIEnableVideoEgress,
			"enable_data_output":   ls.CLI.LivepeerAIEnableDataOutput,
		}

		aiJobSettingsJSON, err := json.Marshal(aiJobSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal AI job params: %w", err)
		}
		aiJobSettingsStr := string(aiJobSettingsJSON)

		aiJobParams := map[string]any{
			"height": ingestHeight,
			"width":  ingestWidth,
		}
		aiJobParamsStr, err := json.Marshal(aiJobParams)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal AI job params: %w", err)
		}

		transcodingConfiguration["aiParams"] = map[string]any{
			"capability":      ls.CLI.LivepeerAICapability,
			"parameters":      string(aiJobSettingsStr),
			"request":         "{}",
			"timeout_seconds": 60,
			"stream_id":       sessionIDRen,
			"params":          string(aiJobParamsStr), //# TODO: add params here
		}

		if ls.CLI.LivepeerAIStreamKey != "" {
			transcodingConfiguration["streamKey"] = ls.CLI.LivepeerAIStreamKey
		}
	}

	log.Debug(ctx, "transcoding configuration", "transcodingConfiguration", transcodingConfiguration)

	bs, err := json.Marshal(transcodingConfiguration)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal livepeer profile: %w", err)
	}

	// Convert MP4 to muxed MPEG-TS
	tsSeg := bytes.Buffer{}
	audioSeg := bytes.Buffer{}
	err = media.MP4ToMPEGTSVideoMP4Audio(ctx, bytes.NewReader(buf), &tsSeg, &audioSeg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert mp4 to ts video/mp4 audio: %w", err)
	}
	if tsSeg.Len() == 0 {
		return nil, fmt.Errorf("no video in segment")
	}
	if audioSeg.Len() == 0 {
		return nil, fmt.Errorf("no audio in segment")
	}

	start := time.Now()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	seqNo := ls.Count
	url_transcode := fmt.Sprintf("%s/live/%s/%d.ts", ls.GatewayURL, sessionIDRen, seqNo)
	ls.Count++

	dur := time.Duration(*spseg.Duration)
	durationMs := int(dur.Milliseconds())

	tsBytes := tsSeg.Bytes()

	req, err := http.NewRequestWithContext(ctx, "POST", url_transcode, bytes.NewReader(tsBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Content-Duration", fmt.Sprintf("%d", durationMs))
	req.Header.Set("Content-Resolution", fmt.Sprintf("%dx%d", ingestWidth, ingestHeight))
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

	resp, err := ls.sendSegmentRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errOut, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateway returned non-OK status (config %s): %d, %s", string(bs), resp.StatusCode, string(errOut))
	}

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
			ctx := log.WithLogValues(ctx, "part", p.FileName())
			if err != nil {
				return nil, fmt.Errorf("failed to get next part: %w", err)
			}

			// Detect and log AI data/text outputs instead of trying to transcode them.
			contentType := p.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "application/json") || strings.HasPrefix(contentType, "text/") {
				body, readErr := io.ReadAll(p)
				if readErr != nil {
					log.Error(ctx, "failed to read ai data output", "error", readErr)
				} else {
					log.Log(ctx, "received ai data output", "contentType", contentType, "length", len(body), "body", string(body))
				}
				continue
			}

			mp4Bs := bytes.Buffer{}
			if ls.CLI.LivepeerDebug {
				debugFile := fmt.Sprintf("%s/livepeer-debug/%s-%06d-output-%s", ls.CLI.DataDir, sessionIDRen, seqNo, p.FileName())
				err = os.WriteFile(debugFile, tsSeg.Bytes(), 0644)
				if err != nil {
					return nil, fmt.Errorf("failed to write debug file: %w", err)
				}
				log.Log(ctx, "wrote debug file", "file", debugFile)
			}
			err = media.MPEGTSToMP4(ctx, p, &mp4Bs)
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
