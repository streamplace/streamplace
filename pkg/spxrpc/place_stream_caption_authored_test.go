package spxrpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/records"
	"stream.place/streamplace/pkg/captions/transcript"
	"stream.place/streamplace/pkg/captions/webvtt"
)

type authoredImportPDS struct{}

func (authoredImportPDS) Do(context.Context, string, string, string, map[string]any, any, any) error {
	return nil
}

func TestAuthoredCaptionImportHTTPRoundTrip(t *testing.T) {
	const srt = "1\n00:00:00,000 --> 00:01:00,000\nVOD caption proof timestamp\n\n2\n00:01:00,000 --> 00:01:30,000\nFirst line. Still here!\nSecond line continues.\n\n3\n00:01:30,010 --> 00:02:00,000\nAfter a ten millisecond gap\n"
	want, err := webvtt.ParseSRT([]byte(srt))
	require.NoError(t, err)
	for _, source := range []captions.Source{captions.SourceImported, captions.SourceHuman} {
		t.Run(string(source), func(t *testing.T) {
			s := capServer(t)
			_, err := (&records.Importer{Store: s.model, Chunking: transcript.ChunkOptions{MaxTimings: 2}}).Import(
				context.Background(), authoredImportPDS{}, "did:plc:owner", records.ImportInput{
					Video: capVideoURI, Language: "en", Format: "srt", Body: srt,
				})
			require.NoError(t, err)
			if source == captions.SourceHuman {
				rows, err := s.model.GetCaptionTranscriptsBySubject(context.Background(), capVideoURI)
				require.NoError(t, err)
				for _, row := range rows {
					rec, err := row.ToRecord()
					require.NoError(t, err)
					rec.Source = string(source)
					require.NoError(t, s.model.UpsertCaptionTranscript(context.Background(), rec, mustATURI(t, row.URI)))
				}
			}
			s.VideoCaptions = &records.Provider{Store: s.model}
			e := echo.New()
			e.GET("/xrpc/place.stream.caption.getCaptions", s.HandleGetCaptions)
			httpServer := httptest.NewServer(e)
			defer httpServer.Close()
			trackID := records.TrackID("did:plc:owner", "en", "captions", string(source))
			// The same provider supplies the native/web VOD overlay.
			overlay, err := s.VideoCaptions.Cues(context.Background(), capVideoURI, trackID)
			require.NoError(t, err)
			require.Len(t, overlay, len(want))
			for i := range want {
				require.Equal(t, want[i].Text, overlay[i].Text)
				require.InDelta(t, want[i].Start.Milliseconds(), overlay[i].Start.Milliseconds(), 1)
				require.InDelta(t, want[i].End.Milliseconds(), overlay[i].End.Milliseconds(), 1)
			}
			for _, format := range []string{"vtt", "srt", "json"} {
				q := url.Values{"video": {capVideoURI}, "track": {trackID}, "format": {format}}
				resp, err := http.Get(httpServer.URL + "/xrpc/place.stream.caption.getCaptions?" + q.Encode())
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
				var got []webvtt.Cue
				switch format {
				case "vtt":
					got, err = webvtt.ParseVTT(body)
					require.NoError(t, err)
				case "srt":
					got, err = webvtt.ParseSRT(body)
					require.NoError(t, err)
				case "json":
					var doc struct {
						Cues []struct {
							Text           string
							StartMs, EndMs int64
						}
					}
					require.NoError(t, json.Unmarshal(body, &doc))
					for _, cue := range doc.Cues {
						got = append(got, webvtt.Cue{Text: cue.Text, Start: time.Duration(cue.StartMs) * time.Millisecond, End: time.Duration(cue.EndMs) * time.Millisecond})
					}
				}
				require.Len(t, got, len(want), format)
				for i := range want {
					require.Equal(t, want[i].Text, got[i].Text, format)
					require.InDelta(t, want[i].Start.Milliseconds(), got[i].Start.Milliseconds(), 1, format)
					require.InDelta(t, want[i].End.Milliseconds(), got[i].End.Milliseconds(), 1, format)
				}
			}
			// HLS selects overlapping authored cues without rewriting their text or span.
			q := url.Values{"video": {capVideoURI}, "track": {trackID + ".vtt"}, "format": {"vtt"}, "start": {"0"}, "end": {"2000"}, "mpegts": {"0"}}
			resp, err := http.Get(httpServer.URL + "/xrpc/place.stream.caption.getCaptions?" + q.Encode())
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, resp.Body.Close())
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Contains(t, string(body), "X-TIMESTAMP-MAP")
			hlsCues, err := webvtt.ParseVTT(body)
			require.NoError(t, err)
			require.Len(t, hlsCues, 1)
			require.Equal(t, want[0].Text, hlsCues[0].Text)
			require.Equal(t, time.Duration(0), hlsCues[0].Start)
			require.InDelta(t, want[0].End.Milliseconds(), hlsCues[0].End.Milliseconds(), 1)
		})
	}
}
