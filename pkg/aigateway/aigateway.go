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
	"net/url"
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

	// Pipeline is the AI pipeline capability name (e.g., "transcriber").
	Pipeline string
}

// Session represents an active AI gateway transcription session.
type Session struct {
	// ID is the unique identifier for this session.
	ID string

	// StopURL is the URL for stopping the session (if provided by gateway).
	StopURL string

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
}

type streamStartRequest struct {
	StreamName string `json:"stream_name"`
	Params     string `json:"params"`
	StreamID   string `json:"stream_id"`
}

type streamStartResponse struct {
	StatusURL string `json:"status_url"`
	DataURL   string `json:"data_url"`
	UpdateURL string `json:"update_url"`
	WhipURL   string `json:"whip_url"`
	WhepURL   string `json:"whep_url"`
	StopURL   string `json:"stop_url"`
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
	base := strings.TrimRight(cfg.BaseURL, "/")
	startCandidates := []string{
		base + "/process/stream/start",
		base + "/ai/stream/start",
	}

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
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	var lastNotFoundStatus string
	var lastNotFoundBody string
	var lastErr error

	for _, startURL := range startCandidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Livepeer", livepeerHeader)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			lastNotFoundStatus = resp.Status
			lastNotFoundBody = string(b)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("start stream failed: %s: %s", resp.Status, string(b))
		}

		var sr streamStartResponse
		if err := json.Unmarshal(b, &sr); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		if sr.StreamID == "" {
			return nil, fmt.Errorf("start response missing stream_id")
		}

		session := &Session{
			ID:        sr.StreamID,
			StopURL:   sr.StopURL,
			StatusURL: sr.StatusURL,
			DataURL:   sr.DataURL,
			UpdateURL: sr.UpdateURL,
			WhipURL:   sr.WhipURL,
			WhepURL:   sr.WhepURL,
		}

		normalizeBase := strings.TrimRight(cfg.BaseURL, "/")
		session.StopURL = normalizeGatewayURL(normalizeBase, session.StopURL)
		session.StatusURL = normalizeGatewayURL(normalizeBase, session.StatusURL)
		session.DataURL = normalizeGatewayURL(normalizeBase, session.DataURL)
		session.UpdateURL = normalizeGatewayURL(normalizeBase, session.UpdateURL)
		session.WhipURL = normalizeGatewayURL(normalizeBase, session.WhipURL)
		session.WhepURL = normalizeGatewayURL(normalizeBase, session.WhepURL)

		return session, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("do request: %w", lastErr)
	}
	return nil, fmt.Errorf("start stream failed: %s: %s", lastNotFoundStatus, lastNotFoundBody)
}

// StopStream terminates an active transcription session.
func StopStream(ctx context.Context, cfg Config, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("empty streamID")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	stopCandidates := []string{
		base + "/process/stream/" + streamID + "/stop",
		base + "/ai/stream/" + streamID + "/stop",
	}

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

	var lastNotFoundStatus string
	var lastNotFoundBody string
	var lastErr error

	for _, stopURL := range stopCandidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, stopURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Livepeer", livepeerHeader)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			lastNotFoundStatus = resp.Status
			lastNotFoundBody = string(b)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("stop stream failed: %s: %s", resp.Status, string(b))
		}
		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("do request: %w", lastErr)
	}
	return fmt.Errorf("stop stream failed: %s: %s", lastNotFoundStatus, lastNotFoundBody)
}

func normalizeGatewayURL(base, raw string) string {
	if raw == "" || base == "" {
		return raw
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return raw
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return raw
	}
	if strings.HasPrefix(raw, "/") {
		ref, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		return baseURL.ResolveReference(ref).String()
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme == "" || u.Host == "" {
		ref, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		return baseURL.ResolveReference(ref).String()
	}
	u.Scheme = baseURL.Scheme
	u.Host = baseURL.Host
	return u.String()
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

	// TimestampUTC is the wall-clock timestamp when the transcript was generated.
	// The SSE format uses an RFC3339 timestamp string.
	TimestampUTC *time.Time `json:"timestamp_utc,omitempty"`

	Timing *Timing `json:"timing,omitempty"`

	// Stats contains optional performance statistics for this transcription.
	Stats *Stats `json:"stats,omitempty"`

	// ReceivedAt is when Streamplace received this event (not from JSON).
	ReceivedAt time.Time `json:"-"`

	// Segments contains the structured transcript payload with explicit media-clock timestamps.
	Segments []TranscriptSegment `json:"segments,omitempty"`
}

func (e *TranscriptEvent) UnmarshalJSON(b []byte) error {
	// Accept the SSE format (timestamp_utc RFC3339 string).
	type rawEvent struct {
		Type         string              `json:"type"`
		TimestampUTC string              `json:"timestamp_utc"`
		Timing       *Timing             `json:"timing"`
		Stats        *Stats              `json:"stats"`
		Segments     []TranscriptSegment `json:"segments"`
	}

	var r rawEvent
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}

	e.Type = r.Type
	e.Timing = r.Timing
	e.Stats = r.Stats
	e.Segments = r.Segments

	if strings.TrimSpace(r.TimestampUTC) == "" {
		return fmt.Errorf("missing timestamp_utc")
	}
	// RFC3339Nano handles both second and sub-second precision.
	ts, err := time.Parse(time.RFC3339Nano, r.TimestampUTC)
	if err != nil {
		return fmt.Errorf("parse timestamp_utc: %w", err)
	}
	e.TimestampUTC = &ts

	return nil
}

// TranscriptSegment represents a timed subtitle unit (phrase/line) in media-clock time.
// Times are stream-relative milliseconds.
type TranscriptSegment struct {
	ID      string          `json:"id"`
	StartMS int64           `json:"start_ms"`
	EndMS   int64           `json:"end_ms"`
	Text    string          `json:"text"`
	Words   []WordTimestamp `json:"words,omitempty"`
}

// WordTimestamp is an optional word-level timing payload.
// Times are stream-relative milliseconds.
type WordTimestamp struct {
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	Text    string `json:"text"`
}

type Timing struct {
	MediaWindowStartMS int64 `json:"media_window_start_ms"`
	MediaWindowEndMS   int64 `json:"media_window_end_ms"`
}

// Stats contains performance statistics for a transcription event.
type Stats struct {
	AudioDurationMS int `json:"audio_duration_ms"`
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

		event, err := parseSSEPayload(data)
		if err != nil {
			log.Debug(ctx, "failed to parse SSE payload", "error", err)
			eventBuf.Reset()
			return
		}

		event.ReceivedAt = time.Now()
		handler(ctx, event)
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

func parseSSEPayload(data string) (TranscriptEvent, error) {
	var outer []string
	if err := json.Unmarshal([]byte(data), &outer); err != nil {
		return TranscriptEvent{}, fmt.Errorf("unmarshal SSE outer payload: %w", err)
	}
	if len(outer) != 1 {
		return TranscriptEvent{}, fmt.Errorf("unexpected SSE outer payload length: %d", len(outer))
	}

	inner := strings.TrimSpace(outer[0])
	if inner == "" {
		return TranscriptEvent{}, fmt.Errorf("empty SSE inner payload")
	}

	var event TranscriptEvent
	if err := json.Unmarshal([]byte(inner), &event); err != nil {
		return TranscriptEvent{}, fmt.Errorf("unmarshal SSE inner transcript event: %w", err)
	}
	return event, nil
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
