package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestHandleAtprotoDID(t *testing.T) {
	const customDID = "did:plc:rbvrr34edl5ddpuwcubjiost"
	tests := []struct {
		name        string
		serverHost  string
		requestHost string
		defaultDID  string
	}{
		{"single node", "stream.place", "stream.place", "did:web:stream.place"},
		{"single node via proxy", "stream.place", "localhost", "did:web:stream.place"},
		{"multi node server", "node.stream.place", "node.stream.place", "did:web:node.stream.place"},
		{"multi node broadcaster", "node.stream.place", "stream.place", "did:web:stream.place"},
		{"multi node via proxy", "node.stream.place", "localhost", "did:web:stream.place"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, override := range []string{"", customDID} {
				name, want := "default", tt.defaultDID
				if override != "" {
					name, want = "override", override
				}
				t.Run(name, func(t *testing.T) {
					a := StreamplaceAPI{CLI: &config.CLI{
						ServerHost:      tt.serverHost,
						BroadcasterHost: "stream.place",
						AtprotoDID:      override,
					}}
					req := httptest.NewRequest(http.MethodGet, "https://"+tt.requestHost+"/.well-known/atproto-did", nil)
					rec := httptest.NewRecorder()
					a.HandleAtprotoDID(t.Context())(rec, req, nil)
					require.Equal(t, http.StatusOK, rec.Code)
					require.Equal(t, want, rec.Body.String())
				})
			}
		})
	}
}
