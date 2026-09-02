package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/llhls"
	"stream.place/streamplace/pkg/media"
)

func TestLLHLSReloadQuery(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantMSN    uint64
		wantPart   uint32
		wantReload bool
	}{
		{name: "absent", query: "", wantReload: false},
		{name: "msn", query: "?_HLS_msn=647", wantMSN: 647, wantReload: true},
		{name: "msn and part", query: "?_HLS_msn=647&_HLS_part=3", wantMSN: 647, wantPart: 3, wantReload: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/playlist.m3u8"+tt.query, nil)
			msn, part, reload, err := llhlsReloadQuery(r)
			if err != nil {
				t.Fatal(err)
			}
			if msn != tt.wantMSN || part != tt.wantPart || reload != tt.wantReload {
				t.Fatalf("got msn=%d part=%d reload=%v, want msn=%d part=%d reload=%v", msn, part, reload, tt.wantMSN, tt.wantPart, tt.wantReload)
			}
		})
	}
}

func TestLLHLSReloadQueryRejectsPartWithoutMSN(t *testing.T) {
	r := httptest.NewRequest("GET", "/playlist.m3u8?_HLS_part=1", nil)
	if _, _, _, err := llhlsReloadQuery(r); err == nil {
		t.Fatal("expected _HLS_part without _HLS_msn to fail")
	}
}

func TestLLHLSMasterAdvertisesVideoMetadata(t *testing.T) {
	master := renderLLHLSMaster("/api/playback/did:plc:test/llhls/rtmp-1", llhls.VideoConfig{
		Codec:  "avc1.64002a",
		Width:  1280,
		Height: 720,
	}, llhls.AudioConfig{Channels: 2})

	for _, want := range []string{
		"#EXT-X-VERSION:10",
		`#EXT-X-STREAM-INF:BANDWIDTH=6500000,CODECS="avc1.64002a,mp4a.40.2",RESOLUTION=1280x720,AUDIO="audio",CLOSED-CAPTIONS=NONE`,
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="default",DEFAULT=YES,AUTOSELECT=YES,CHANNELS="2",CODECS="mp4a.40.2",URI="/api/playback/did:plc:test/llhls/rtmp-1/audio/index.m3u8"`,
		`/api/playback/did:plc:test/llhls/rtmp-1/video/index.m3u8`,
	} {
		if !strings.Contains(master, want) {
			t.Errorf("master missing %q:\n%s", want, master)
		}
	}
	if strings.Contains(master, "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("RTMP master cannot make a presentation-wide independence guarantee:\n%s", master)
	}
}

func TestLLHLSMasterAdvertisesAudioOnlyVariant(t *testing.T) {
	master := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{Codec: "avc1.64002a", Width: 1280, Height: 720}, llhls.AudioConfig{Channels: 2})
	want := "#EXT-X-STREAM-INF:BANDWIDTH=128000,CODECS=\"mp4a.40.2\"\n/api/playback/test/audio/index.m3u8"
	if !strings.Contains(master, want) {
		t.Fatalf("master missing audio-only variant %q:\n%s", want, master)
	}
}

func TestLLHLSMasterOmitsIndependentSegments(t *testing.T) {
	master := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{}, llhls.AudioConfig{Channels: 2})
	if strings.Contains(master, "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("master advertised independent segments without metadata:\n%s", master)
	}
}

func TestLLHLSMasterAdvertisesAudioChannels(t *testing.T) {
	master := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{}, llhls.AudioConfig{Channels: 1})
	if !strings.Contains(master, `CHANNELS="1"`) {
		t.Fatalf("master omitted mono audio metadata:\n%s", master)
	}
}

func TestLLHLSMasterRedirectsWhilePresentationIsInitializing(t *testing.T) {
	const user = "did:key:z6MkPreInitTest"
	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: llhls.NewWindow()})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	handler := api.HandleLLHLSMaster(context.Background())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/master.m3u8", nil)
	handler(recorder, request, httprouter.Params{{Key: "user", Value: user}})

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("pre-init master status = %d, want %d", recorder.Code, http.StatusTemporaryRedirect)
	}
	wantLocation := "/xrpc/place.stream.playback.getLivePlaylist?streamer=" + url.QueryEscape(user)
	if got := recorder.Header().Get("Location"); got != wantLocation {
		t.Fatalf("pre-init master location = %q, want %q", got, wantLocation)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("pre-init master cache policy = %q, want no-store", got)
	}
}

func TestLLHLSMasterWaitsForBothRenditions(t *testing.T) {
	const user = "did:key:z6MkBothTracksTest"
	window := llhls.NewWindow()
	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "video", Generation: 1, Data: []byte("video-init")}); err != nil {
		t.Fatal(err)
	}
	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	handler := api.HandleLLHLSMaster(context.Background())

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/master.m3u8", nil), httprouter.Params{{Key: "user", Value: user}})
	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("master with video-only init status = %d, want %d", recorder.Code, http.StatusTemporaryRedirect)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("master with video-only init cache policy = %q, want no-store", got)
	}

	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "audio", Generation: 1, Data: []byte("audio-init")}); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/master.m3u8", nil), httprouter.Params{{Key: "user", Value: user}})
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `AUDIO="audio"`) {
		t.Fatalf("master after both init segments = status %d body %q", recorder.Code, recorder.Body.String())
	}
}

func TestLLHLSAudioPlaylistUsesCompleteSegments(t *testing.T) {
	const user = "did:key:z6MkAudioSegmentsOnlyTest"
	window := llhls.NewWindow()
	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "audio", Generation: 1, Data: []byte("audio-init")}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Part: 0, Duration: time.Second, Data: []byte("part-0")}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Part: 1, Duration: time.Second, Data: []byte("part-1")}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.SegmentComplete, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Duration: 2 * time.Second, Data: []byte("segment")}); err != nil {
		t.Fatal(err)
	}

	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	params := httprouter.Params{
		{Key: "user", Value: user},
		{Key: "presentation", Value: "p"},
		{Key: "track", Value: "audio"},
	}
	recorder := httptest.NewRecorder()
	api.HandleLLHLSPlaylist(context.Background())(recorder, httptest.NewRequest(http.MethodGet, "/audio/index.m3u8", nil), params)
	if recorder.Code != http.StatusOK {
		t.Fatalf("audio playlist status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"#EXT-X-PART-INF:", "#EXT-X-SERVER-CONTROL:", "#EXT-X-PART:", "#EXT-X-PRELOAD-HINT:", "#EXT-X-RENDITION-REPORT:"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("audio playlist contains %q:\n%s", forbidden, body)
		}
	}
	if !strings.Contains(body, "#EXTINF:2.000000,") || !strings.Contains(body, "/audio/1.m4s") {
		t.Fatalf("audio playlist omitted complete segment:\n%s", body)
	}
}

func TestLLHLSPartHandlerKeepsExactURIIdentity(t *testing.T) {
	const (
		user         = "did:key:z6MkTest"
		presentation = "p"
		track        = "video"
	)
	window := llhls.NewWindow()
	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: presentation, Track: track, Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: presentation, Track: track, Generation: 1, MSN: 4, Part: 0, Data: []byte("parent")}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.SegmentComplete, Presentation: presentation, Track: track, Generation: 1, MSN: 4, Data: []byte("segment")}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: presentation, Track: track, Generation: 1, MSN: 5, Part: 0, Data: []byte("next")}); err != nil {
		t.Fatal(err)
	}

	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	handler := api.HandleLLHLSPart(context.Background())
	params := httprouter.Params{
		{Key: "user", Value: user},
		{Key: "presentation", Value: presentation},
		{Key: "track", Value: track},
		{Key: "msn", Value: "4"},
		{Key: "part.m4s", Value: "0.m4s"},
	}
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/part.m4s", nil), params)
	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), []byte("parent")) {
		t.Fatalf("exact part response = status %d body %q", recorder.Code, recorder.Body.Bytes())
	}

	params = append(params[:len(params)-1], httprouter.Param{Key: "part.m4s", Value: "1.m4s"})
	recorder = httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/part.m4s", nil), params)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("invalid exact part status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("invalid exact part cache policy = %q, want no-store", got)
	}
}

func TestLLHLSPartHandlerUsesAudioMIMEType(t *testing.T) {
	const user = "did:key:z6MkAudioMIMETypeTest"
	window := llhls.NewWindow()
	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "audio", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: "p", Track: "audio", Generation: 1, MSN: 1, Part: 0, Data: []byte("audio-part")}); err != nil {
		t.Fatal(err)
	}

	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	params := httprouter.Params{
		{Key: "user", Value: user},
		{Key: "presentation", Value: "p"},
		{Key: "track", Value: "audio"},
		{Key: "msn", Value: "1"},
		{Key: "part.m4s", Value: "0.m4s"},
	}

	recorder := httptest.NewRecorder()
	api.HandleLLHLSPart(context.Background())(recorder, httptest.NewRequest(http.MethodGet, "/part.m4s", nil), params)
	if recorder.Code != http.StatusOK {
		t.Fatalf("audio part status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "audio/mp4" {
		t.Fatalf("audio part content type = %q, want audio/mp4", got)
	}
}

func TestLLHLSPartHandlerWaitsForExactPartThenServesIt(t *testing.T) {
	const user = "did:key:z6MkWaitTest"
	window := llhls.NewWindow()
	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "video", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	handler := api.HandleLLHLSPart(context.Background())
	params := httprouter.Params{
		{Key: "user", Value: user},
		{Key: "presentation", Value: "p"},
		{Key: "track", Value: "video"},
		{Key: "msn", Value: "7"},
		{Key: "part.m4s", Value: "0.m4s"},
	}
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler(recorder, httptest.NewRequest(http.MethodGet, "/part.m4s", nil), params)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("exact part request resolved before its part was published")
	case <-time.After(10 * time.Millisecond):
	}
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: "p", Track: "video", Generation: 1, MSN: 7, Part: 0, Data: []byte("part")}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("exact part request did not resolve after publication")
	}
	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), []byte("part")) {
		t.Fatalf("published part response = status %d body %q", recorder.Code, recorder.Body.Bytes())
	}
}

func setLLWindowsForTest(manager *media.MediaManager, windows map[string]*llhls.Window) {
	field := reflect.ValueOf(manager).Elem().FieldByName("llWindows")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(windows))
}

func TestMissingLLHLSMediaIsNotCached(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/part.m4s", nil)

	serveLLHLSBytes(recorder, request, nil, "part.m4s", true, "video/mp4")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing media status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("missing media cache policy = %q, want no-store", got)
	}
}
