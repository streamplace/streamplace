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

	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
)

type LivepeerSession struct {
	SessionID  string
	Count      int
	GatewayURL string
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

func (ls *LivepeerSession) PostSegmentToGateway(ctx context.Context, buf []byte) ([][]byte, error) {
	url := fmt.Sprintf("%s/live/%s/%d.mp4", ls.GatewayURL, ls.SessionID, ls.Count)
	ls.Count++

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "multipart/mixed")

	resp, err := ctxhttp.Do(ctx, &aqhttp.Client, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send segment to gateway: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned non-OK status: %d", resp.StatusCode)
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
