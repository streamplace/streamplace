package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"stream.place/streamplace/pkg/config"
	_ "stream.place/streamplace/pkg/media/mediatesting"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/notifications"
)

func TestRedirectHandler(t *testing.T) {
	tests := []struct {
		name        string
		httpAddr    string
		httpsAddr   string
		requestURL  string
		expectedURL string
	}{
		{
			name:        "default https port",
			httpAddr:    "0.0.0.0:80",
			httpsAddr:   "0.0.0.0:443",
			requestURL:  "http://example.com/",
			expectedURL: "https://example.com/",
		},
		{
			name:        "non-default https port",
			httpAddr:    "0.0.0.0:80",
			httpsAddr:   "0.0.0.0:8443",
			requestURL:  "http://example.com/",
			expectedURL: "https://example.com:8443/",
		},
		{
			name:        "non-default http port",
			httpAddr:    "0.0.0.0:8080",
			httpsAddr:   "0.0.0.0:443",
			requestURL:  "http://example.com:8080/",
			expectedURL: "https://example.com/",
		},
		{
			name:        "non-default both",
			httpAddr:    "0.0.0.0:8080",
			httpsAddr:   "0.0.0.0:8443",
			requestURL:  "http://example.com:8080/",
			expectedURL: "https://example.com:8443/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &config.CLI{HTTPAddr: tt.httpAddr, HTTPSAddr: tt.httpsAddr}
			mod := &model.DBModel{}
			a := StreamplaceAPI{CLI: cli, Model: mod}

			handler, err := a.RedirectHandler(context.Background())
			assert.NoError(t, err, "RedirectHandler should not return an error")

			req := httptest.NewRequest("GET", tt.requestURL, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			result := rr.Result()
			assert.Equal(t, http.StatusTemporaryRedirect, result.StatusCode, "handler returned wrong status code")

			redirectURL, err := result.Location()
			assert.NoError(t, err, "Failed to get redirect location")

			assert.Equal(t, tt.expectedURL, redirectURL.String(), "handler returned unexpected redirect URL")
		})
	}
}

type MockFirebase struct {
}

func (m *MockFirebase) Blast(ctx context.Context, nots []string, nb *notifications.NotificationBlast) error {
	return nil
}
