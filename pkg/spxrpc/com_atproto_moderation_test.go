package spxrpc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/lestrrat-go/jwx/v2/jwk"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/test"
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
	chat := strings.Replace(record, "place.stream.livestream", "place.stream.chat.message", 1)
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
		indexedChat     bool
		streamerHandle  string
		missingStreamer bool
		wantReason      string
		withClip        bool
		withoutClip     bool
		mismatchedCID   bool
	}{
		{name: "account uses first configured labeler", subject: account, labelers: []string{labeler, "did:plc:other"}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "record uses configured labeler", subject: record, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "unindexed chat message is still reported", subject: chat, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "chat identifies streamer", subject: chat, indexedChat: true, streamerHandle: "streamer.test", labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler", wantReason: "in chat of @streamer.test (did:plc:streamer)"},
		{name: "chat preserves comment", subject: chat, reason: "report details", indexedChat: true, streamerHandle: "streamer.test", labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler", wantReason: "report details\n\nin chat of @streamer.test (did:plc:streamer)"},
		{name: "chat includes clip evidence", subject: chat, reason: "report details", indexedChat: true, streamerHandle: "streamer.test", withClip: true, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler", wantReason: "report details\n\nin chat of @streamer.test (did:plc:streamer)"},
		{name: "chat preserves maximum length comment", subject: chat, reason: strings.Repeat("x", 2000), indexedChat: true, streamerHandle: "streamer.test", withClip: true, withoutClip: true, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "chat preserves maximum byte comment", subject: chat, reason: "a" + strings.Repeat("\u0301", 9999) + "a", indexedChat: true, streamerHandle: "streamer.test", withoutClip: true, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "chat counts graphemes in comment", subject: chat, reason: strings.Repeat("e\u0301", 1800), indexedChat: true, streamerHandle: "streamer.test", labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler", wantReason: strings.Repeat("e\u0301", 1800) + "\n\nin chat of @streamer.test (did:plc:streamer)"},
		{name: "chat with another indexed version", subject: chat, reason: "report details", indexedChat: true, mismatchedCID: true, streamerHandle: "streamer.test", labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler"},
		{name: "chat with unknown streamer handle", subject: chat, indexedChat: true, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler", wantReason: "in chat of did:plc:streamer"},
		{name: "chat with unindexed streamer", subject: chat, indexedChat: true, missingStreamer: true, labelers: []string{labeler}, wantStatus: http.StatusOK, wantProxy: labeler + "#atproto_labeler", wantReason: "in chat of did:plc:streamer"},
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
			mm := &media.MediaManager{}
			streamerDID := "did:plc:streamer"
			if tc.withClip {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				key, err := secp256k1.GeneratePrivateKey()
				require.NoError(t, err)
				signer, err := media.MakeMediaSigner(ctx, cli, "streamer.test", key.ToECDSA(), nil)
				require.NoError(t, err)
				streamerDID = signer.DID()
				localSigner := signer.(*media.MediaSignerLocal)
				keyPEM, err := signers.MarshalES256KPrivateKeyPEM(localSigner.Signer)
				require.NoError(t, err)
				manifest := []byte(fmt.Sprintf(`{"title":"report clip","assertions":[{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}},{"label":"cawg.metadata","data":{"@context":{"dc":"http://purl.org/dc/elements/1.1/"},"dc:creator":%q,"dc:title":"report clip","dc:date":%q}}]}`, streamerDID, time.Now().UTC().Format(time.RFC3339Nano)))
				cfg := media.IngestWorkerConfig{
					StreamerDID: streamerDID, KeyPEM: keyPEM, CertPEM: localSigner.Cert, Manifest: manifest,
					NodeKeyPEM: keyPEM, NodeCertPEM: localSigner.Cert, BroadcasterHost: "stream.example",
				}
				fixture, err := test.Files.ReadFile("fixtures/h264-opus-frag.mp4")
				require.NoError(t, err)
				cli.AllowedStreams = []string{streamerDID}
				cli.BroadcasterHost = "stream.example"
				mm, err = media.MakeMediaManager(ctx, cli, nil, nil, nil, nil, nil)
				require.NoError(t, err)
				// The MP4 ingest worker takes H264+AAC; the fixture has Opus audio.
				pipeline, err := gst.NewPipelineFromString("appsrc name=source ! qtdemux name=d d. ! queue ! h264parse ! mp4mux name=mux fragment-duration=500 ! appsink name=sink d. ! queue ! opusdec ! audioconvert ! audioresample ! fdkaacenc ! aacparse ! mux.")
				require.NoError(t, err)
				defer func() { require.NoError(t, pipeline.SetState(gst.StateNull)) }()
				source, err := pipeline.GetElementByName("source")
				require.NoError(t, err)
				app.SrcFromElement(source).SetCallbacks(&app.SourceCallbacks{NeedDataFunc: media.ReaderNeedDataIncremental(ctx, bytes.NewReader(fixture))})
				sink, err := pipeline.GetElementByName("sink")
				require.NoError(t, err)
				var mp4 bytes.Buffer
				app.SinkFromElement(sink).SetCallbacks(&app.SinkCallbacks{NewSampleFunc: media.WriterNewSample(ctx, &mp4)})
				busErr := make(chan error, 1)
				go func() { busErr <- media.HandleBusMessages(ctx, pipeline) }()
				require.NoError(t, pipeline.SetState(gst.StatePlaying))
				require.NoError(t, <-busErr)
				var frames bytes.Buffer
				require.NoError(t, media.RunMP4IngestWorker(ctx, cfg, bytes.NewReader(mp4.Bytes()), ingestframe.NewWriter(&frames), func() []byte { return manifest }))
				reader := ingestframe.NewReader(&frames)
				for {
					typ, segment, err := reader.ReadFrame()
					if errors.Is(err, io.EOF) {
						break
					}
					require.NoError(t, err)
					require.Equal(t, ingestframe.Segment, typ)
					require.NoError(t, mm.ValidateMP4(ctx, bytes.NewReader(segment), true))
				}
			}
			mod := newTestModel(t)
			if tc.indexedChat {
				require.NoError(t, mod.UpdateRepo(&model.Repo{DID: "did:plc:subject", Handle: "chatter.test"}))
				if !tc.missingStreamer {
					require.NoError(t, mod.UpdateRepo(&model.Repo{DID: streamerDID, Handle: tc.streamerHandle}))
				}
				cid := "bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku"
				if tc.mismatchedCID {
					cid = "bafy-another-version"
				}
				require.NoError(t, mod.CreateChatMessage(context.Background(), &model.ChatMessage{
					CID: cid, URI: "at://did:plc:subject/place.stream.chat.message/1", RepoDID: "did:plc:subject", StreamerRepoDID: streamerDID,
				}))
			}
			s, err := NewServer(context.Background(), cli, mod, nil, op, middleware.New(middleware.Config{}), nil, nil, nil, mm, nil, nil, nil, nil)
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
			var expectedBody map[string]any
			require.NoError(t, json.Unmarshal([]byte(body), &expectedBody))
			expectedBody["reason"] = tc.reason
			if tc.wantReason != "" {
				expectedBody["reason"] = strings.ReplaceAll(tc.wantReason, "did:plc:streamer", streamerDID)
			}
			clips, err := filepath.Glob(filepath.Join(cli.DataDir, "*", "clips", "*.mp4"))
			require.NoError(t, err)
			if tc.withClip && !tc.withoutClip {
				require.Len(t, clips, 1)
				clip, err := os.ReadFile(clips[0])
				require.NoError(t, err)
				require.NotEmpty(t, clip)
				expectedBody["reason"] = fmt.Sprintf("%s\n\nClip: https://stream.example/api/clip/%s/%s", expectedBody["reason"], streamerDID, filepath.Base(clips[0]))
			} else {
				require.Empty(t, clips, "failed clip attempts must not leave files behind")
			}
			expectedJSON, err := json.Marshal(expectedBody)
			require.NoError(t, err)
			require.JSONEq(t, string(expectedJSON), string(report.body))
			if tc.withClip || tc.withoutClip || tc.subject == chat && (!tc.indexedChat || tc.mismatchedCID) {
				require.NotContains(t, logs.String(), "failed to make clip for report", "unknown chat streamer must not use the chatter's stream")
			} else {
				require.Contains(t, logs.String(), "failed to make clip for report")
			}
			if tc.wantStatus == http.StatusOK {
				var output indigoatproto.ModerationCreateReport_Output
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &output))
				require.EqualValues(t, 42, output.Id)
			}
		})
	}
}
