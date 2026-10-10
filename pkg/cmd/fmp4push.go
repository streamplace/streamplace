package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
)

// pushFMP4 streams file to the node at endpoint the way the Streamplace OBS
// plugin does: it asks /api/ingest/client-configuration for the FMP4 ingest
// endpoint, then POSTs the file there as realtime-paced fragmented MP4 with a
// chunked body, with AAC audio like OBS's default encoder. It returns when the
// file ends, the node hangs up, or ctx is done.
func pushFMP4(ctx context.Context, endpoint, streamKey, file string) error {
	ingestURL, err := fmp4IngestURL(ctx, endpoint, streamKey)
	if err != nil {
		return err
	}

	gstinit.InitGST()
	// identity sync=true paces each track at realtime, so the push plays as a
	// live stream instead of arriving in one burst.
	pipeline, err := gst.NewPipelineFromString(strings.Join([]string{
		"filesrc name=filesrc ! qtdemux name=demux",
		"demux.video_0 ! queue ! identity sync=true ! h264parse ! mux.",
		"demux.audio_0 ! queue ! identity sync=true ! opusdec ! audioconvert ! audioresample ! fdkaacenc ! aacparse ! mux.",
		"mp4mux name=mux fragment-duration=100 ! appsink name=sink sync=false",
	}, "\n"))
	if err != nil {
		return err
	}
	defer func() { _ = pipeline.BlockSetState(gst.StateNull) }()
	fileSrc, err := pipeline.GetElementByName("filesrc")
	if err != nil {
		return err
	}
	if err := fileSrc.Set("location", file); err != nil {
		return err
	}
	sink, err := pipeline.GetElementByName("sink")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	body, bodyWriter := io.Pipe()
	app.SinkFromElement(sink).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: media.WriterNewSample(ctx, bodyWriter),
	})

	pushErr := make(chan error, 1)
	go func() {
		req, err := http.NewRequestWithContext(ctx, "POST", ingestURL, body)
		if err == nil {
			req.Header.Set("Content-Type", "video/mp4")
			var resp *http.Response
			if resp, err = http.DefaultClient.Do(req); err == nil {
				if resp.StatusCode != http.StatusOK {
					msg, _ := io.ReadAll(resp.Body)
					err = fmt.Errorf("node answered %s: %s", resp.Status, strings.TrimSpace(string(msg)))
				}
				resp.Body.Close()
			}
		}
		// A node that hangs up mid-stream stops the pipeline too.
		_ = body.CloseWithError(err)
		cancel()
		pushErr <- err
	}()

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return err
	}
	pipelineErr := media.HandleBusMessages(ctx, pipeline)
	_ = bodyWriter.CloseWithError(pipelineErr)
	if err := <-pushErr; err != nil {
		return fmt.Errorf("push to %s: %w", strings.ReplaceAll(ingestURL, streamKey, "{stream_key}"), err)
	}
	return pipelineErr
}

// fmp4IngestURL asks the node for the plugin's ingest URL.
func fmp4IngestURL(ctx context.Context, endpoint, streamKey string) (string, error) {
	reqBody, err := json.Marshal(map[string]string{"authentication": streamKey})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/api/ingest/client-configuration", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var config struct {
		Status struct {
			Result   string `json:"result"`
			HTMLEnUS string `json:"html_en_us"`
		} `json:"status"`
		IngestEndpoints []struct {
			Protocol    string `json:"protocol"`
			URLTemplate string `json:"url_template"`
		} `json:"ingest_endpoints"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return "", fmt.Errorf("client configuration (HTTP %d): %w", resp.StatusCode, err)
	}
	if config.Status.Result == "error" {
		return "", fmt.Errorf("client configuration refused: %s", config.Status.HTMLEnUS)
	}
	for _, e := range config.IngestEndpoints {
		if e.Protocol == "FMP4" {
			return strings.ReplaceAll(e.URLTemplate, "{stream_key}", streamKey), nil
		}
	}
	return "", fmt.Errorf("client configuration offers no FMP4 ingest endpoint")
}
