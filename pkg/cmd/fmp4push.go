package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
)

// pushFMP4Tracks labels the tracks pushFMP4's muxer creates (in pad order) the
// way the OBS plugin labels a full mix on mixer Track 1 plus a stem named
// "Music" on Track 2: an AAC and an Opus encode of each, sharing a group.
var pushFMP4Tracks = map[uint32]muxl.TrackMeta{
	2: {AlternateGroup: 1, Label: "Track 1"},
	3: {AlternateGroup: 1, Label: "Track 1"},
	4: {AlternateGroup: 2, Label: "Music"},
	5: {AlternateGroup: 2, Label: "Music"},
}

// pushFMP4 streams file to the node at endpoint the way the Streamplace OBS
// plugin does with one stem: it asks /api/ingest/client-configuration for the
// FMP4 ingest endpoint, then POSTs realtime-paced fragmented MP4 with a
// chunked body. The file's audio is the full mix, and a copy of it is the
// "Music" stem, each as AAC and Opus. It returns when the file ends, the
// node hangs up, or ctx is done.
func pushFMP4(ctx context.Context, endpoint, streamKey, file string) error {
	ingestURL, err := fmp4IngestURL(ctx, endpoint, streamKey)
	if err != nil {
		return err
	}

	gstinit.InitGST()
	// identity sync=true paces each track at realtime, so the push plays as a
	// live stream instead of arriving in one burst. mp4mux numbers tracks in
	// pad order, which pushFMP4Tracks relies on.
	launched, err := gst.ParseLaunch(strings.Join([]string{
		"filesrc name=filesrc ! qtdemux name=demux",
		"mp4mux name=mux fragment-duration=100 ! appsink name=sink sync=false",
		"demux.video_0 ! queue ! identity sync=true ! h264parse ! mux.video_0",
		"demux.audio_0 ! queue ! identity sync=true ! tee name=mix",
		"mix. ! queue ! opusdec ! audioconvert ! audioresample ! tee name=pcm",
		"pcm. ! queue ! fdkaacenc ! aacparse ! mux.audio_0",
		"mix. ! queue ! opusparse ! mux.audio_1",
		"pcm. ! queue ! fdkaacenc ! aacparse ! mux.audio_2",
		"pcm. ! queue ! opusenc ! opusparse ! mux.audio_3",
	}, "\n"))
	if err != nil {
		return err
	}
	pipeline, ok := launched.(gst.Pipeline)
	if !ok {
		return fmt.Errorf("fmp4 push: launch did not produce a pipeline")
	}
	defer func() { _ = pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone) }()
	fileSrc := pipeline.GetByName("filesrc")
	if fileSrc == nil {
		return fmt.Errorf("fmp4 push: filesrc element not found")
	}
	fileSrc.SetObjectProperty("location", file)
	sink := pipeline.GetByName("sink")
	if sink == nil {
		return fmt.Errorf("fmp4 push: sink element not found")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	muxed, muxedWriter := io.Pipe()
	sink.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: media.WriterNewSample(ctx, muxedWriter),
	})
	body, bodyWriter := io.Pipe()
	go func() { _ = bodyWriter.CloseWithError(labelTracks(ctx, muxed, bodyWriter)) }()

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
		_ = muxed.CloseWithError(err)
		cancel()
		pushErr <- err
	}()

	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return fmt.Errorf("fmp4 push: set playing: state change failed")
	}
	pipelineErr := media.HandleBusMessages(ctx, pipeline)
	_ = muxedWriter.CloseWithError(pipelineErr)
	if err := <-pushErr; err != nil {
		return fmt.Errorf("push to %s: %w", strings.ReplaceAll(ingestURL, streamKey, "{stream_key}"), err)
	}
	return pipelineErr
}

// labelTracks re-emits an fMP4 stream through muxl with pushFMP4Tracks applied,
// so its init carries the groups and titles the plugin would write: the
// canonical init, then each GoP's track segments. It stops when in closes:
// muxl's segmenter must not see a cancelled context (see media.signSegments).
func labelTracks(ctx context.Context, in io.Reader, out io.Writer) error {
	ctx = context.WithoutCancel(ctx)
	events := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- muxl.RunMuxlSegmenterEvents(ctx, in, events, muxl.WithSegmentTrackMeta(pushFMP4Tracks))
		close(events)
	}()
	var writeErr error
	for ev := range events {
		if writeErr != nil {
			continue // keep draining so the segmenter can finish
		}
		switch ev.Type {
		case "init":
			_, writeErr = out.Write(ev.Data)
		case "segment":
			ids := make([]int, 0, len(ev.Tracks))
			for k := range ev.Tracks {
				id, _ := strconv.Atoi(k)
				ids = append(ids, id)
			}
			slices.Sort(ids)
			for _, id := range ids {
				if _, writeErr = out.Write(ev.Tracks[strconv.Itoa(id)]); writeErr != nil {
					break
				}
			}
		}
	}
	if err := <-errCh; err != nil {
		return err
	}
	return writeErr
}

// fmp4IngestURL asks the node for the plugin's ingest URL.
func fmp4IngestURL(ctx context.Context, endpoint, streamKey string) (string, error) {
	// Named mixer tracks are how a streamer asks for stems; see obs-plugin/README.md.
	reqBody, err := json.Marshal(map[string]any{
		"authentication": streamKey,
		"preferences": map[string]any{"audio_tracks": []map[string]any{
			{"track_id": 0, "name": ""},
			{"track_id": 1, "name": "Music"},
		}},
	})
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
