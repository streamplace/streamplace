package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"aquareum.tv/aquareum/pkg/config"
	"aquareum.tv/aquareum/pkg/crypto/signers/eip712"
	"aquareum.tv/aquareum/pkg/crypto/signers/eip712/eip712test"
	_ "aquareum.tv/aquareum/pkg/media/mediatesting"
	"aquareum.tv/aquareum/pkg/model"
	"aquareum.tv/aquareum/pkg/notifications"
	v0 "aquareum.tv/aquareum/pkg/schema/v0"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			cli := &config.CLI{HttpAddr: tt.httpAddr, HttpsAddr: tt.httpsAddr}
			mod := &model.DBModel{}
			a := AquareumAPI{CLI: cli, Model: mod}

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

func (m *MockFirebase) Blast(ctx context.Context, nots []model.Notification, nb *notifications.NotificationBlast) error {
	return nil
}

func TestGoLiveHandler(t *testing.T) {
	mod, err := model.MakeDB("sqlite://:memory:")
	require.NoError(t, err)
	eip712test.WithTestSigner(func(signer *eip712.EIP712Signer) {
		tests := []struct {
			adminAccount string
			responseCode int
			name         string
		}{
			{
				name:         "successful auth",
				adminAccount: signer.Opts.EthAccountAddr,
				responseCode: 204,
			},
			{
				name:         "failed auth",
				adminAccount: "0x156118110DcD4b7c91fC1F4200691d4b6e3BcaF7",
				responseCode: 403,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cli := &config.CLI{AdminAccount: tt.adminAccount, FirebaseServiceAccount: "foo"}
				a := AquareumAPI{CLI: cli, Model: mod, Signer: signer, FirebaseNotifier: &MockFirebase{}}
				handler := a.HandleIdentityPUT(context.Background())

				goLive := v0.Identity{
					DID:    "did:plc:dkh4rwafdcda4ko7lewe43ml",
					Handle: "@aquareum.bsky.social",
				}
				signed, err := signer.SignMessage(goLive)
				require.NoError(t, err)

				req := httptest.NewRequest("PUT", "https://aquareum.tv/api/settings/example", bytes.NewReader(signed))
				rr := httptest.NewRecorder()

				handler(rr, req, httprouter.Params{{Key: "id", Value: "example"}})
				require.Equal(t, tt.responseCode, rr.Code)
			})
		}
	})
}
