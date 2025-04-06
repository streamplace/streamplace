package livepeer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

type LivepeerSession struct {
	SessionID  string
	Count      int
	GatewayURL string
	SegLock    sync.Mutex
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

func NewLivepeerSession(ctx context.Context, did string, gatewayURL string) (*LivepeerSession, error) {
	sessionID := RandomTrailer(8)
	return &LivepeerSession{
		SessionID:  fmt.Sprintf("%s-%s", did, sessionID),
		Count:      0,
		GatewayURL: gatewayURL,
	}, nil
}

func (ls *LivepeerSession) PostSegmentToGateway(ctx context.Context, buf []byte, seg *streamplace.Segment) ([][]byte, error) {
	ctx = log.WithLogValues(ctx, "func", "PostSegmentToGateway")
	ls.SegLock.Lock()
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()
	url := fmt.Sprintf("%s/live/%s/%d.mp4", ls.GatewayURL, ls.SessionID, ls.Count)
	ls.Count++

	dur := time.Duration(*seg.Duration)
	durationMs := int(dur.Milliseconds())
	log.Debug(ctx, "posting segment to livepeer gateway", "duration_ms", durationMs, "url", url)

	vid := seg.Video[0]
	width := int(vid.Width)
	height := int(vid.Height)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		ls.SegLock.Unlock()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Content-Duration", fmt.Sprintf("%d", durationMs))
	req.Header.Set("Content-Resolution", fmt.Sprintf("%dx%d", width, height))

	resp, err := ctxhttp.Do(ctx, &aqhttp.Client, req)
	if err != nil {
		ls.SegLock.Unlock()
		return nil, fmt.Errorf("failed to send segment to gateway: %w", err)
	}
	ls.SegLock.Unlock()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errOut, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateway returned non-OK status: %d, %s", resp.StatusCode, string(errOut))
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
			if err != nil {
				return nil, fmt.Errorf("failed to get next part: %w", err)
			}
			bs, err := io.ReadAll(p)
			if err != nil {
				return nil, fmt.Errorf("failed to read part: %w", err)
			}
			log.Debug(ctx, "got part back from livepeer gateway", "length", len(bs), "name", p.FileName())
			out = append(out, bs)
		}
	}

	return out, nil
}
