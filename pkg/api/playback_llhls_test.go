package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
	})

	for _, want := range []string{
		"#EXT-X-VERSION:10",
		`#EXT-X-STREAM-INF:BANDWIDTH=2500000,CODECS="avc1.64002a,mp4a.40.2",RESOLUTION=1280x720,CLOSED-CAPTIONS=NONE`,
		`/api/playback/did:plc:test/llhls/rtmp-1/video/index.m3u8`,
	} {
		if !strings.Contains(master, want) {
			t.Errorf("master missing %q:\n%s", want, master)
		}
	}
	if strings.Contains(master, "#EXT-X-MEDIA:TYPE=AUDIO") || strings.Contains(master, `AUDIO="audio"`) {
		t.Fatalf("muxed master should not advertise a separate audio rendition:\n%s", master)
	}
	if strings.Contains(master, "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("RTMP master cannot make a presentation-wide independence guarantee:\n%s", master)
	}
}

func TestLLHLSMasterOmitsIndependentSegments(t *testing.T) {
	master := renderLLHLSMaster("/api/playback/test", llhls.VideoConfig{})
	if strings.Contains(master, "#EXT-X-INDEPENDENT-SEGMENTS") {
		t.Fatalf("master advertised independent segments without metadata:\n%s", master)
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

	serveLLHLSBytes(recorder, request, nil, "part.m4s", true)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing media status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("missing media cache policy = %q, want no-store", got)
	}
}
