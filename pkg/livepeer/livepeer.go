package livepeer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
)

type LivepeerSession struct {
	SessionID string
	Count     int
}

func NewLivepeerSession(ctx context.Context) (*LivepeerSession, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}
	sessionID := uu.String()
	return &LivepeerSession{
		SessionID: sessionID,
		Count:     0,
	}, nil
}

func (ls *LivepeerSession) PostSegmentToGateway(ctx context.Context, buf []byte) ([]byte, error) {
	url := fmt.Sprintf("http://127.0.0.1:9999/live/%s/%d.mp4", ls.SessionID, ls.Count)
	ls.Count++

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "multipart/mixed")

	resp, err := aqhttp.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send segment to gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned non-OK status: %d", resp.StatusCode)
	}

	var slurp []byte

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
			slurp, err = io.ReadAll(p)
			if err != nil {
				return nil, fmt.Errorf("failed to read part: %w", err)
			}
			log.Log(ctx, "part", "length", len(slurp))
		}
	}

	fd, err := os.Create(fmt.Sprintf("/home/iameli/testvids/livepeer-element/%s-%08d.mp4", ls.SessionID, ls.Count))
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer fd.Close()

	fd.Write(slurp)

	return slurp, nil
}
