package api

import (
	"net/http/httptest"
	"testing"
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
