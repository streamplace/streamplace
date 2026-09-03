package api

import (
	"bytes"
	"context"
	"fmt"
	"math"
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
	master, err := renderLLHLSMaster("/api/playback/did:plc:test/llhls/rtmp-1", llhls.VideoConfig{
		Codec:            "avc1.64002a",
		Width:            1280,
		Height:           720,
		FrameRate:        30,
		Bandwidth:        5000000,
		AverageBandwidth: 4000000,
	}, llhls.AudioConfig{Channels: 2, Bandwidth: 128000, AverageBandwidth: 128000})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"#EXT-X-VERSION:10",
		`#EXT-X-STREAM-INF:BANDWIDTH=5128000,AVERAGE-BANDWIDTH=4128000,CODECS="avc1.64002a,mp4a.40.2",RESOLUTION=1280x720,FRAME-RATE=30.000,AUDIO="audio",CLOSED-CAPTIONS=NONE`,
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

func TestLLHLSMasterOmitsAudioOnlyVariant(t *testing.T) {
	video := llhls.VideoConfig{Codec: "avc1.64002a", Width: 1280, Height: 720, FrameRate: 30, Bandwidth: 5000000, AverageBandwidth: 4000000}
	audio := llhls.AudioConfig{Channels: 2, Bandwidth: 96000, AverageBandwidth: 88000}
	master, err := renderLLHLSMaster("/api/playback/test", video, audio)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(master, "/api/playback/test/audio/index.m3u8\n") {
		t.Fatalf("Apple master advertised an audio-only variant:\n%s", master)
	}
}

func TestLLHLSPlaylistRejectsReloadTooFarAhead(t *testing.T) {
	const user = "did:key:z6MkFutureReloadTest"
	window := llhls.NewWindow()
	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "video", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: "p", Track: "video", Generation: 1, MSN: 7, Part: 0, Duration: time.Second, Data: []byte("part")}); err != nil {
		t.Fatal(err)
	}

	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	params := httprouter.Params{
		{Key: "user", Value: user},
		{Key: "presentation", Value: "p"},
		{Key: "track", Value: "video"},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/video/index.m3u8?_HLS_msn=10", nil)
	api.HandleLLHLSPlaylist(context.Background())(recorder, request, params)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("future reload status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLLHLSMasterReturnsUnavailableWithoutMeasuredBandwidth(t *testing.T) {
	const user = "did:key:z6MkMissingBandwidthTest"
	window := llhls.NewWindow()
	window.SetVideoConfig(llhls.VideoConfig{FrameRate: 30, Bandwidth: 5000000, AverageBandwidth: 4000000})
	window.SetAudioConfig(llhls.AudioConfig{Channels: 2})
	for _, track := range []string{"video", "audio"} {
		if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: track, Generation: 1, Data: []byte(track + "-init")}); err != nil {
			t.Fatal(err)
		}
	}

	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	recorder := httptest.NewRecorder()
	requestContext, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/master.m3u8", nil).WithContext(requestContext)
	api.HandleLLHLSMaster(context.Background())(recorder, request, httprouter.Params{{Key: "user", Value: user}})
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("master without measured bandwidth status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestLLHLSMasterRedirectsForInvalidVideoFrameRate(t *testing.T) {
	for _, fps := range []float64{math.NaN(), math.Inf(1), -1, 61} {
		t.Run(fmt.Sprintf("fps-%v", fps), func(t *testing.T) {
			const user = "did:key:z6MkInvalidFrameRateTest"
			window := llhls.NewWindow()
			window.SetVideoConfig(llhls.VideoConfig{FrameRate: fps, Bandwidth: 5000000, AverageBandwidth: 4000000})
			window.SetAudioConfig(llhls.AudioConfig{Channels: 2, Bandwidth: 128000, AverageBandwidth: 128000})
			for _, track := range []string{"video", "audio"} {
				if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: track, Generation: 1, Data: []byte(track + "-init")}); err != nil {
					t.Fatal(err)
				}
			}
			manager := &media.MediaManager{}
			setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
			api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
			recorder := httptest.NewRecorder()
			api.HandleLLHLSMaster(context.Background())(recorder, httptest.NewRequest(http.MethodGet, "/master.m3u8", nil), httprouter.Params{{Key: "user", Value: user}})
			if recorder.Code != http.StatusTemporaryRedirect {
				t.Fatalf("master with invalid frame rate status = %d, want %d", recorder.Code, http.StatusTemporaryRedirect)
			}
		})
	}
}

func TestLLHLSMasterAdvertisesFrameRateWhenKnown(t *testing.T) {
	master, err := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{
		Codec:            "avc1.64002a",
		Width:            1280,
		Height:           720,
		FrameRate:        59.94,
		Bandwidth:        5000000,
		AverageBandwidth: 4000000,
	}, llhls.AudioConfig{Channels: 2, Bandwidth: 128000, AverageBandwidth: 128000})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(master, "FRAME-RATE=59.940") {
		t.Fatalf("master omitted known video frame rate:\n%s", master)
	}
}

func TestLLHLSMasterOmitsIndependentSegments(t *testing.T) {
	master, err := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{FrameRate: 30, Bandwidth: 5000000, AverageBandwidth: 4000000}, llhls.AudioConfig{Channels: 2, Bandwidth: 128000, AverageBandwidth: 128000})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(master, "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("master advertised independent segments without metadata:\n%s", master)
	}
}

func TestLLHLSMasterAdvertisesAudioChannels(t *testing.T) {
	master, err := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{FrameRate: 30, Bandwidth: 5000000, AverageBandwidth: 4000000}, llhls.AudioConfig{Channels: 1, Bandwidth: 128000, AverageBandwidth: 128000})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(master, `CHANNELS="1"`) {
		t.Fatalf("master omitted mono audio metadata:\n%s", master)
	}
}

func TestLLHLSMasterRendererRejectsIncompleteMetadata(t *testing.T) {
	if _, err := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{}, llhls.AudioConfig{}); err == nil {
		t.Fatal("renderer accepted incomplete metadata")
	}
}

func TestLLHLSMasterBitrateSumSaturates(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	if got := sumBitrates(maxInt-1, 2); got != maxInt {
		t.Fatalf("sumBitrates overflowed to %d, want %d", got, maxInt)
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
	window.SetVideoConfig(llhls.VideoConfig{FrameRate: 30, Bandwidth: 5000000, AverageBandwidth: 4000000})
	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	handler := api.HandleLLHLSMaster(context.Background())

	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		handler(recorder, httptest.NewRequest(http.MethodGet, "/master.m3u8", nil), httprouter.Params{{Key: "user", Value: user}})
		result <- recorder
	}()
	select {
	case recorder := <-result:
		t.Fatalf("master with video-only init returned early with status %d", recorder.Code)
	case <-time.After(10 * time.Millisecond):
	}

	if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "audio", Generation: 1, Data: []byte("audio-init")}); err != nil {
		t.Fatal(err)
	}
	window.SetAudioConfig(llhls.AudioConfig{Channels: 2, Bandwidth: 128000, AverageBandwidth: 128000})
	var recorder *httptest.ResponseRecorder
	select {
	case recorder = <-result:
	case <-time.After(time.Second):
		t.Fatal("master did not become ready after both renditions initialized")
	}
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `AUDIO="audio"`) {
		t.Fatalf("master after both init segments = status %d body %q", recorder.Code, recorder.Body.String())
	}
}

func TestLLHLSMasterRedirectsWithoutVideoFrameRate(t *testing.T) {
	const user = "did:key:z6MkMissingFrameRateTest"
	window := llhls.NewWindow()
	for _, track := range []string{"video", "audio"} {
		if err := window.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: track, Generation: 1, Data: []byte(track + "-init")}); err != nil {
			t.Fatal(err)
		}
	}

	manager := &media.MediaManager{}
	setLLWindowsForTest(manager, map[string]*llhls.Window{user: window})
	api := &StreamplaceAPI{MediaManager: manager, Aliases: map[string]string{}}
	recorder := httptest.NewRecorder()
	requestContext, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/master.m3u8", nil).WithContext(requestContext)
	api.HandleLLHLSMaster(context.Background())(recorder, request, httprouter.Params{{Key: "user", Value: user}})
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("master without frame rate status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestLLHLSAudioPlaylistUsesLLHLSParts(t *testing.T) {
	const user = "did:key:z6MkAudioLLHLSTest"
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
	if err := window.Observe(llhls.Event{Kind: llhls.Part, Presentation: "p", Track: "audio", Generation: 1, MSN: 2, Part: 0, Duration: time.Second, Data: []byte("next-part")}); err != nil {
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
	for _, required := range []string{"#EXT-X-PART-INF:", "#EXT-X-SERVER-CONTROL:", `#EXT-X-PART:DURATION=1.000000,URI="/api/playback/did:key:z6MkAudioLLHLSTest/llhls/p/audio/2/0.m4s"`, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="/api/playback/did:key:z6MkAudioLLHLSTest/llhls/p/audio/2/1.m4s"`} {
		if !strings.Contains(body, required) {
			t.Errorf("audio playlist missing %q:\n%s", required, body)
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

	params[1].Value = "stale-presentation"
	recorder = httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/part.m4s", nil), params)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("stale presentation part status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("stale presentation part cache policy = %q, want no-store", got)
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
