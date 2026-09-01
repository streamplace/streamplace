package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stream.place/streamplace/pkg/llhls"
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
		"#EXT-X-INDEPENDENT-SEGMENTS",
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
