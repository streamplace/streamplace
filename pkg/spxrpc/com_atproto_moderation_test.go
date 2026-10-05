package spxrpc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/lestrrat-go/jwx/v2/jwk"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/media"
)

func TestCreateReportRouting(t *testing.T) {
	verbosity := flag.Lookup("v")
	previousVerbosity := verbosity.Value.String()
	require.NoError(t, verbosity.Value.Set("3"))
	t.Cleanup(func() { require.NoError(t, verbosity.Value.Set(previousVerbosity)) })

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jwkKey, err := jwk.FromRaw(key)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(jwkKey)
	require.NoError(t, err)

	var logs bytes.Buffer
	logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(logger) })
	gormLogger := config.GormLogger
	config.GormLogger = slogGorm.New(slogGorm.WithHandler(slog.Default().Handler()))
	t.Cleanup(func() { config.GormLogger = gormLogger })

	const labeler = "did:plc:streamplacelabeler"
	const account = `{"$type":"com.atproto.admin.defs#repoRef","did":"did:plc:subject"}`
	const record = `{"$type":"com.atproto.repo.strongRef","uri":"at://did:plc:subject/place.stream.livestream/1","cid":"bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku"}`
	for _, tc := range []struct {
		name            string
		subject         string
		reason          string
		proxy           string
		labelers        []string
		unauthenticated bool
		upstreamStatus  int
		wantStatus      int
		wantProxy       string
		wantError       string
	}{
		{name: "account uses first configured labeler", subject: account, labelers: []string{labeler, "did:plc:other"}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "record uses configured labeler", subject: record, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "unindexed chat message is still reported", subject: strings.Replace(record, "place.stream.livestream", "place.stream.chat.message", 1), labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "comment is preserved", subject: account, reason: "report details", labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "explicit destination overrides default", subject: account, proxy: "did:plc:chosen#atproto_labeler", labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: "did:plc:chosen#atproto_labeler"},
		{name: "explicit destination without default", subject: account, proxy: "did:plc:chosen#atproto_labeler", wantStatus: http.StatusOK, wantProxy: "did:plc:chosen#atproto_labeler"},
		{name: "missing destination", subject: account, wantStatus: http.StatusBadRequest, wantError: "Atproto-Proxy header is required"},
		{name: "missing session", subject: account, labelers: []string{labeler}, unauthenticated: true, wantStatus: http.StatusUnauthorized, wantError: "oauth session not found"},
		{name: "missing subject", subject: "null", labelers: []string{labeler}, wantStatus: http.StatusBadRequest, wantError: "subject is required"},
		{name: "invalid account", subject: `{"$type":"com.atproto.admin.defs#repoRef","did":"invalid"}`, labelers: []string{labeler}, wantStatus: http.StatusBadRequest, wantError: "invalid subject did"},
		{name: "invalid record", subject: `{"$type":"com.atproto.repo.strongRef","uri":"invalid","cid":"bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku"}`, labelers: []string{labeler}, wantStatus: http.StatusBadRequest, wantError: "invalid subject uri"},
		{name: "malformed body", subject: "{", labelers: []string{labeler}, wantStatus: http.StatusBadRequest},
		{name: "upstream failure is surfaced", subject: account, labelers: []string{labeler}, upstreamStatus: http.StatusBadRequest, wantStatus: http.StatusInternalServerError, wantProxy: labeler + "#atproto_labeler", wantError: "labeler unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			type forwardedReport struct {
				method, path, proxy string
				body                []byte
				err                 error
			}
			reports := make(chan forwardedReport, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				reports <- forwardedReport{r.Method, r.URL.Path, r.Header.Get("Atproto-Proxy"), body, err}
				w.Header().Set("Content-Type", "application/json")
				if tc.upstreamStatus != 0 {
					w.WriteHeader(tc.upstreamStatus)
					_, _ = io.WriteString(w, `{"error":"Unavailable","message":"labeler unavailable"}`)
					return
				}
				_, _ = io.WriteString(w, `{"id":42,"reasonType":"com.atproto.moderation.defs#reasonOther","reportedBy":"did:plc:reporter","createdAt":"2026-10-04T00:00:00Z","subject":`+tc.subject+`}`)
			}))
			t.Cleanup(upstream.Close)

			op := oatproxy.New(&oatproxy.Config{Host: "stream.example"})
			cli := &config.CLI{DataDir: t.TempDir(), Labelers: tc.labelers}
			s, err := NewServer(context.Background(), cli, newTestModel(t), nil, op, middleware.New(middleware.Config{}), nil, nil, nil, &media.MediaManager{}, nil, nil, nil, nil)
			require.NoError(t, err)

			body := `{"reasonType":"com.atproto.moderation.defs#reasonOther","subject":` + tc.subject + `}`
			if tc.reason != "" {
				body = strings.TrimSuffix(body, "}") + `,"reason":"` + tc.reason + `"}`
			}
			req := httptest.NewRequest(http.MethodPost, "/xrpc/com.atproto.moderation.createReport", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Atproto-Proxy", tc.proxy)
			if !tc.unauthenticated {
				session := &oatproxy.OAuthSession{DID: "did:plc:reporter", PDSUrl: upstream.URL, UpstreamAccessToken: "test-token", UpstreamDPoPPrivateJWK: string(keyJSON)}
				ctx := context.WithValue(req.Context(), oatproxy.OATProxyContextKey, op)
				ctx = context.WithValue(ctx, oatproxy.OAuthSessionContextKey, session)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			if tc.wantError != "" {
				require.Contains(t, rec.Body.String(), tc.wantError)
			}
			if tc.wantProxy == "" {
				require.Empty(t, reports)
				return
			}
			require.Len(t, reports, 1)
			report := <-reports
			require.NoError(t, report.err)
			require.Equal(t, http.MethodPost, report.method)
			require.Equal(t, "/xrpc/com.atproto.moderation.createReport", report.path)
			require.Equal(t, tc.wantProxy, report.proxy)
			expectedBody := body
			if tc.reason == "" {
				expectedBody = strings.TrimSuffix(body, "}") + `,"reason":""}`
			}
			require.JSONEq(t, expectedBody, string(report.body))
			require.Contains(t, logs.String(), "failed to make clip for report")
			if tc.wantStatus == http.StatusOK {
				var output indigoatproto.ModerationCreateReport_Output
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &output))
				require.EqualValues(t, 42, output.Id)
			}
		})
	}
}
