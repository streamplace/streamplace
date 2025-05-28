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
	"strings"
	"time"

	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/streamplace"
)

const SegmentsInFlight = 2

type LivepeerSession struct {
	SessionID  string
	Count      int
	GatewayURL string
	Guard      chan struct{}
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
		Guard:      make(chan struct{}, SegmentsInFlight),
	}, nil
}

func (ls *LivepeerSession) PostSegmentToGateway(ctx context.Context, buf []byte, spseg *streamplace.Segment, rs renditions.Renditions) ([][]byte, error) {
	ctx = log.WithLogValues(ctx, "func", "PostSegmentToGateway")
	lpProfiles := rs.ToLivepeerProfiles()
	transcodingConfiguration := map[string]any{
		"manifestID": ls.SessionID,
		"profiles":   lpProfiles,
	}
	bs, err := json.Marshal(transcodingConfiguration)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal livepeer profile: %w", err)
	}
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
	ls.Guard <- struct{}{}
	start := time.Now()
	// check if context is done since we were waiting for the lock
	if ctx.Err() != nil {
		<-ls.Guard
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()
	seqNo := ls.Count
	url := fmt.Sprintf("%s/live/%s/%d.ts", ls.GatewayURL, ls.SessionID, seqNo)
	ls.Count++

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

	resp, err := ctxhttp.Do(ctx, &aqhttp.Client, req)
	if err != nil {
		<-ls.Guard
		return nil, fmt.Errorf("failed to send segment to gateway (config %s): %w", string(bs), err)
	}
	<-ls.Guard
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
			mp4Bs := bytes.Buffer{}
			audioReader := bytes.NewReader(audioSeg.Bytes())
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
