// Package aigateway provides client functionality for communicating with
// AI transcription gateways. It handles session management, media publishing
// via RTMP or WHIP, and SSE-based transcript event streaming.
package aigateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"stream.place/streamplace/pkg/log"
)

const (
	// DefaultStreamTimeout is the default timeout for AI gateway stream sessions.
	DefaultStreamTimeout = 120

	// StopStreamTimeout is the timeout for stopping a stream session.
	StopStreamTimeout = 5

	// maxResponseBodySize limits how much of an error response body we read.
	maxResponseBodySize = 8 << 10 // 8KB

	// sseBufferSize is the initial buffer size for SSE scanning.
	sseBufferSize = 64 * 1024

	// sseMaxBufferSize is the maximum buffer size for SSE scanning.
	sseMaxBufferSize = 1024 * 1024
)

// Config holds the configuration for connecting to an AI gateway.
type Config struct {
	// BaseURL is the base URL of the AI gateway (e.g., "http://localhost:5937").
	BaseURL string

	// PathPrefix is an optional path prefix for gateway requests (e.g., "gateway").
	PathPrefix string

	// RewriteURLsTo rewrites returned URLs to use this base for local access.
	RewriteURLsTo string

	// Pipeline is the AI pipeline capability name (e.g., "transcriber").
	Pipeline string

	// RTMPHost is the host:port for RTMP media ingress if not provided by gateway.
	RTMPHost string
}

// Session represents an active AI gateway transcription session.
type Session struct {
	// ID is the unique identifier for this session.
	ID string

	// StatusURL is the URL to check session status.
	StatusURL string

	// DataURL is the SSE endpoint for receiving transcript events.
	DataURL string

	// UpdateURL is the URL for sending session updates.
	UpdateURL string

	// WhipURL is the WHIP endpoint for WebRTC media ingress (if available).
	WhipURL string

	// WhepURL is the WHEP endpoint for WebRTC media egress (if available).
	WhepURL string

	// RTMPURL is the RTMP endpoint for media ingress (if available).
	RTMPURL string
}

type streamStartRequest struct {
	StreamName string `json:"stream_name"`
	Params     string `json:"params"`
	StreamID   string `json:"stream_id"`
	RTMPOutput string `json:"rtmp_output"`
}

type streamStartResponse struct {
	StatusURL string `json:"status_url"`
	DataURL   string `json:"data_url"`
	UpdateURL string `json:"update_url"`
	WhipURL   string `json:"whip_url"`
	WhepURL   string `json:"whep_url"`
	RTMPURL   string `json:"rtmp_url"`
	StreamID  string `json:"stream_id"`
}

type startParams struct {
	EnableVideoIngress bool `json:"enable_video_ingress"`
	EnableVideoEgress  bool `json:"enable_video_egress"`
	EnableDataOutput   bool `json:"enable_data_output"`
}

// envelope wraps request parameters in the Livepeer gateway header format.
type envelope struct {
	Request        string `json:"request"`
	ParametersJSON string `json:"parameters"`
	Capability     string `json:"capability"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// StartStream initiates a new transcription session with the AI gateway.
// It returns a Session containing the endpoints for media ingress and transcript output.
func StartStream(ctx context.Context, cfg Config, streamName string) (*Session, error) {
	prefix := ""
	if cfg.PathPrefix != "" {
		prefix = "/" + strings.Trim(cfg.PathPrefix, "/")
	}
	startURL := strings.TrimRight(cfg.BaseURL, "/") + prefix + "/ai/stream/start"

	env := envelope{
		Request:        "{}",
		ParametersJSON: mustJSON(startParams{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}),
		Capability:     cfg.Pipeline,
		TimeoutSeconds: DefaultStreamTimeout,
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	livepeerHeader := base64.StdEncoding.EncodeToString(envBytes)

	paramsObj := map[string]any{
		"height": 720,
		"width":  1280,
	}
	paramsJSON := mustJSON(paramsObj)

	body := streamStartRequest{
		StreamName: streamName,
		Params:     paramsJSON,
		StreamID:   "",
		RTMPOutput: "",
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Livepeer", livepeerHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		return nil, fmt.Errorf("start stream failed: %s: %s", resp.Status, string(b))
	}

	var sr streamStartResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if sr.StreamID == "" {
		return nil, fmt.Errorf("start response missing stream_id")
	}

	session := &Session{
		ID:        sr.StreamID,
		StatusURL: sr.StatusURL,
		DataURL:   sr.DataURL,
		UpdateURL: sr.UpdateURL,
		WhipURL:   sr.WhipURL,
		WhepURL:   sr.WhepURL,
		RTMPURL:   sr.RTMPURL,
	}

	if cfg.RewriteURLsTo != "" {
		session.StatusURL = rewriteURL(session.StatusURL, cfg.RewriteURLsTo)
		session.DataURL = rewriteURL(session.DataURL, cfg.RewriteURLsTo)
		session.UpdateURL = rewriteURL(session.UpdateURL, cfg.RewriteURLsTo)
		session.WhipURL = rewriteURL(session.WhipURL, cfg.RewriteURLsTo)
		session.WhepURL = rewriteURL(session.WhepURL, cfg.RewriteURLsTo)
		session.RTMPURL = rewriteURL(session.RTMPURL, cfg.RewriteURLsTo)
	}

	return session, nil
}

// StopStream terminates an active transcription session.
func StopStream(ctx context.Context, cfg Config, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("empty streamID")
	}

	prefix := ""
	if cfg.PathPrefix != "" {
		prefix = "/" + strings.Trim(cfg.PathPrefix, "/")
	}
	stopURL := strings.TrimRight(cfg.BaseURL, "/") + prefix + "/ai/stream/" + streamID + "/stop"

	env := envelope{
		Request:        mustJSON(map[string]string{"stream_id": streamID}),
		ParametersJSON: mustJSON(map[string]any{}),
		Capability:     cfg.Pipeline,
		TimeoutSeconds: StopStreamTimeout,
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	livepeerHeader := base64.StdEncoding.EncodeToString(envBytes)

	bodyObj := map[string]string{"stream_id": streamID}
	bodyBytes, err := json.Marshal(bodyObj)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stopURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Livepeer", livepeerHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		return fmt.Errorf("stop stream failed: %s: %s", resp.Status, string(b))
	}

	return nil
}

// ConstructRTMPURL builds an RTMP URL using the provided host and the session ID.
func (s *Session) ConstructRTMPURL(rtmpHost string) string {
	return fmt.Sprintf("rtmp://%s/%s", rtmpHost, s.ID)
}

func rewriteURL(original, newBase string) string {
	if original == "" || newBase == "" {
		return original
	}
	idx := strings.Index(original, "://")
	if idx == -1 {
		return original
	}
	rest := original[idx+3:]
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		return newBase
	}
	path := rest[slashIdx:]
	path = strings.TrimPrefix(path, "/gateway")
	return strings.TrimRight(newBase, "/") + path
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TranscriptEvent represents a single transcription event from the AI gateway.
type TranscriptEvent struct {
	// Type is the event type (e.g., "transcript").
	Type string `json:"type"`

	// TimestampMS is the timestamp in milliseconds when the transcript was generated.
	TimestampMS int64 `json:"timestamp_ms"`

	// CycleID identifies the transcription cycle this event belongs to.
	CycleID string `json:"cycle_id"`

	// Text is the transcribed text content.
	Text string `json:"text"`

	// Stats contains optional performance statistics for this transcription.
	Stats *Stats `json:"stats,omitempty"`

	// ReceivedAt is when Streamplace received this event (not from JSON).
	ReceivedAt time.Time `json:"-"`
}

// Stats contains performance statistics for a transcription event.
type Stats struct {
	FrameCount      int     `json:"frame_count"`
	AudioDurationMS int     `json:"audio_duration_ms"`
	MaxNewTokens    int     `json:"max_new_tokens"`
	TimingsMS       Timings `json:"timings_ms"`
}

// Timings contains timing information for transcription generation.
type Timings struct {
	Generate int `json:"generate"`
}

// EventHandler is a callback function for processing transcript events.
type EventHandler func(ctx context.Context, event TranscriptEvent)

// ReadSSE connects to the SSE data stream and invokes handler for each transcript event.
// It blocks until the context is cancelled or the stream ends.
func ReadSSE(ctx context.Context, dataURL string, handler EventHandler) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dataURL, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		return fmt.Errorf("data stream failed: %s: %s", resp.Status, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, sseBufferSize)
	scanner.Buffer(buf, sseMaxBufferSize)

	var eventBuf strings.Builder

	flushEvent := func() {
		if eventBuf.Len() == 0 {
			return
		}
		data := strings.TrimSpace(eventBuf.String())
		if data == "" {
			eventBuf.Reset()
			return
		}

		events, err := parseSSEPayload(data)
		if err != nil {
			log.Debug(ctx, "failed to parse SSE payload", "error", err, "data", data)
			eventBuf.Reset()
			return
		}

		for _, event := range events {
			event.ReceivedAt = time.Now()
			handler(ctx, event)
		}
		eventBuf.Reset()
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			flushEvent()
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(line[len("data:"):])
			if eventBuf.Len() > 0 {
				eventBuf.WriteByte('\n')
			}
			eventBuf.WriteString(payload)
		} else if strings.TrimSpace(line) == "" {
			flushEvent()
		}
	}

	if err := scanner.Err(); err != nil && !isContextError(err, ctx) {
		return fmt.Errorf("scanner error: %w", err)
	}

	flushEvent()
	return nil
}

func parseSSEPayload(data string) ([]TranscriptEvent, error) {
	var outer []string
	if err := json.Unmarshal([]byte(data), &outer); err != nil {
		var single TranscriptEvent
		if err2 := json.Unmarshal([]byte(data), &single); err2 == nil {
			return []TranscriptEvent{single}, nil
		}
		return nil, fmt.Errorf("unmarshal outer array: %w", err)
	}

	var events []TranscriptEvent
	for _, s := range outer {
		var event TranscriptEvent
		if err := json.Unmarshal([]byte(s), &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

func isContextError(err error, ctx context.Context) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil {
		return true
	}
	if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}
	return false
}
