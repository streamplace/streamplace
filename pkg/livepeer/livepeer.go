package livepeer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
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
	SessionID  string
	Count      int
	GatewayURL string
	Guard      chan struct{}
	CLI        *config.CLI
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

// PostAISegmentToGateway sends the segment to the unified transcode endpoint and returns both
// AI stream URLs (from JSON body) and transcoded renditions (from multipart body).
func (ls *LivepeerSession) PostAISegmentToGateway(ctx context.Context, buf []byte, spseg *streamplace.Segment, rs renditions.Renditions) (*StreamUrls, [][]byte, error) {
	ctx = log.WithLogValues(ctx, "func", "PostAISegmentToGateway")
	start := time.Now()
	lpProfiles := rs.ToLivepeerProfiles()
	sessionIDRen := fmt.Sprintf("%s-%dren", ls.SessionID, len(rs))
	transcodingConfiguration := map[string]any{
		"manifestID": sessionIDRen,
		"profiles":   lpProfiles,
	}

	vid := spseg.Video[0]
	ingestWidth := int(vid.Width)
	ingestHeight := int(vid.Height)
	if ingestWidth == 0 || ingestHeight == 0 {
		log.Debug(ctx, "video resolution not available in segment metadata", "width", ingestWidth, "height", ingestHeight, "creator", spseg.Creator)
	}

	if ls.CLI.LivepeerAIProcessing {
		aiJobSettings := map[string]any{
			"enable_video_ingress": ls.CLI.LivepeerAIEnableVideoIngress,
			"enable_video_egress":  ls.CLI.LivepeerAIEnableVideoEgress,
			"enable_data_output":   ls.CLI.LivepeerAIEnableDataOutput,
		}

		aiJobSettingsJSON, err := json.Marshal(aiJobSettings)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal AI job params: %w", err)
		}
		aiJobSettingsStr := string(aiJobSettingsJSON)

		aiJobParams := map[string]any{
			"height": ingestHeight,
			// "prompts": string(promptsJSONString),
			"width": ingestWidth,
		}
		aiJobParamsStr, err := json.Marshal(aiJobParams)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal AI job params: %w", err)
		}

		log.Debug(ctx, "ai job params", "aiJobParams", aiJobParams)

		requestMap := map[string]any{}
		requestJSON, err := json.Marshal(requestMap)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		transcodingConfiguration["aiParams"] = map[string]any{
			"capability":      ls.CLI.LivepeerAICapability,
			"parameters":      string(aiJobSettingsStr),
			"request":         string(requestJSON),
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
		return nil, nil, fmt.Errorf("failed to marshal livepeer profile: %w", err)
	}

	// Convert MP4 to muxed MPEG-TS (aligns with transcode ingest format expected by /process/transcode)
	tsSeg := bytes.Buffer{}
	_, err = media.MP4ToMPEGTS(ctx, bytes.NewReader(buf), &tsSeg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert mp4 to ts for ai: %w", err)
	}
	if tsSeg.Len() == 0 {
		return nil, nil, fmt.Errorf("no data in segment for ai")
	}

	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	gatewayURL := strings.TrimSuffix(ls.GatewayURL, "/")
	seqNo := ls.Count
	url_ai := fmt.Sprintf("%s/process/transcode/%s/%d.ts", gatewayURL, sessionIDRen, seqNo)
	log.Debug(ctx, "sending AI segment", "url", url_ai)
	ls.Count++

	dur := time.Duration(*spseg.Duration)
	durationMs := int(dur.Milliseconds())

	tsBytes := tsSeg.Bytes()

	req_ai, err := http.NewRequestWithContext(ctx, "POST", url_ai, bytes.NewReader(tsBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AI request: %w", err)
	}
	req_ai.Header.Set("Accept", "multipart/mixed")
	req_ai.Header.Set("Content-Type", "video/MP2T")
	req_ai.Header.Set("Content-Duration", fmt.Sprintf("%d", durationMs))
	req_ai.Header.Set("Content-Resolution", fmt.Sprintf("%dx%d", ingestWidth, ingestHeight))
	req_ai.Header.Set("Livepeer-Transcode-Configuration", string(bs))

	// Enforce a cap on concurrent in-flight segment posts.
	select {
	case ls.Guard <- struct{}{}:
		defer func() { <-ls.Guard }()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	resp_ai, err := ls.sendSegmentRequest(ctx, req_ai)
	if err != nil {
		return nil, nil, err
	}
	defer resp_ai.Body.Close()

	if resp_ai.StatusCode != http.StatusOK {
		errOut, _ := io.ReadAll(resp_ai.Body)
		return nil, nil, fmt.Errorf("gateway (ai) returned non-OK status (config %s): %d, %s", string(bs), resp_ai.StatusCode, string(errOut))
	}

	var streamUrls *StreamUrls
	out := [][]byte{}

	// Check for X-AI-Stream-Urls header (base64 encoded JSON)
	aiStreamUrlsHeader := resp_ai.Header.Get("X-AI-Stream-Urls")
	if aiStreamUrlsHeader != "" {
		urlsBytes, err := base64.StdEncoding.DecodeString(aiStreamUrlsHeader)
		if err == nil {
			var urls StreamUrls
			if err := json.Unmarshal(urlsBytes, &urls); err == nil {
				streamUrls = &urls
				log.Log(ctx, "✓ GOT AI STREAM URLS FROM X-AI-Stream-Urls HEADER",
					"data_url", urls.DataURL, "stream_id", urls.StreamID,
					"whip_url", urls.WhipURL, "whep_url", urls.WhepURL,
					"rtmp_url", urls.RtmpURL, "update_url", urls.UpdateURL)
			} else {
				log.Error(ctx, "failed to parse AI stream URLs from header", "error", err)
			}
		} else {
			log.Error(ctx, "failed to decode AI stream URLs header", "error", err)
		}
	}

	mediaType, params, err := mime.ParseMediaType(resp_ai.Header.Get("Content-Type"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse media type: %w", err)
	}

	if strings.HasPrefix(mediaType, "application/json") {
		bodyBytes, err := io.ReadAll(resp_ai.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read ai response body: %w", err)
		}
		var urls StreamUrls
		if err := json.Unmarshal(bodyBytes, &urls); err == nil {
			streamUrls = &urls
			log.Log(ctx, "✓ GOT AI STREAM URLS FROM /process/transcode BODY",
				"data_url", urls.DataURL, "stream_id", urls.StreamID,
				"whip_url", urls.WhipURL, "whep_url", urls.WhepURL,
				"rtmp_url", urls.RtmpURL, "update_url", urls.UpdateURL)
		} else {
			return nil, nil, fmt.Errorf("failed to parse ai response body as json: %w", err)
		}
	} else if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(resp_ai.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get next part: %w", err)
			}

			// Detect AI data/text outputs and attempt to parse stream URLs if present.
			contentType := p.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "application/json") || strings.HasPrefix(contentType, "text/") {
				body, readErr := io.ReadAll(p)
				if readErr != nil {
					log.Error(ctx, "failed to read ai data output", "error", readErr)
				} else {
					log.Log(ctx, "received ai data output", "contentType", contentType, "length", len(body), "body", string(body))
					var urls StreamUrls
					if jsonErr := json.Unmarshal(body, &urls); jsonErr == nil && (urls.StreamID != "" || urls.DataURL != "") {
						streamUrls = &urls
						log.Log(ctx, "✓ GOT AI STREAM URLS FROM multipart BODY",
							"data_url", urls.DataURL, "stream_id", urls.StreamID,
							"whip_url", urls.WhipURL, "whep_url", urls.WhepURL,
							"rtmp_url", urls.RtmpURL, "update_url", urls.UpdateURL)
					}
				}
				continue
			}

			partBytes, readErr := io.ReadAll(p)
			if readErr != nil {
				log.Error(ctx, "failed to read gateway multipart part", "error", readErr)
				continue
			}
			if len(partBytes) == 0 {
				log.Error(ctx, "empty gateway multipart part")
				continue
			}

			mp4Bs := bytes.Buffer{}
			if ls.CLI.LivepeerDebug {
				debugFile := fmt.Sprintf("%s/livepeer-debug/%s-output-%s", ls.CLI.DataDir, sessionIDRen, p.FileName())
				err = os.WriteFile(debugFile, partBytes, 0644)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to write debug file: %w", err)
				}
				log.Log(ctx, "wrote debug file", "file", debugFile)
			}

			// The gateway may return either MPEG-TS or MP4 parts depending on config/version.
			// Handle MP4 directly; only transmux when it's actually TS.
			lcCT := strings.ToLower(contentType)
			if strings.HasPrefix(lcCT, "video/mp4") || strings.HasPrefix(lcCT, "application/mp4") {
				out = append(out, partBytes)
				log.Debug(ctx, "got mp4 part back from livepeer gateway", "length", len(partBytes), "name", p.FileName())
				continue
			}

			err = media.MPEGTSToMP4(ctx, bytes.NewReader(partBytes), &mp4Bs)
			if err != nil {
				log.Error(ctx, "failed to convert ts to mp4", "error", err)
				continue
			}
			bs := mp4Bs.Bytes()
			log.Debug(ctx, "got part back from livepeer gateway", "length", len(bs), "name", p.FileName())
			out = append(out, bs)
		}
	}

	if streamUrls == nil {
		log.Debug(ctx, "no AI stream URLs found in /process/transcode response body")
	}

	spmetrics.TranscodeDuration.WithLabelValues(spseg.Creator).Observe(float64(time.Since(start).Milliseconds()))
	return streamUrls, out, nil
}
