package spxrpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestStationCreatorScopeBeforeClaim(t *testing.T) {
	const (
		did          = "did:plc:creator"
		full         = "atproto include:place.stream.authFull"
		target       = "repo?collection=place.stream.multistream.target"
		draft        = "repo?collection=place.stream.vod.draftVideo"
		video        = "repo?collection=place.stream.video"
		boundSpace   = "space:place.stream.personal?authority=" + did + "&skey=self"
		spaceTarget  = boundSpace + "&collection=place.stream.multistream.target"
		spaceDraft   = boundSpace + "&collection=place.stream.vod.draftMetadata"
		createTarget = "/xrpc/place.stream.multistream.createTarget"
		putTarget    = "/xrpc/place.stream.multistream.putTarget"
		deleteTarget = "/xrpc/place.stream.multistream.deleteTarget"
		listTargets  = "/xrpc/place.stream.multistream.listTargets"
		createDraft  = "/xrpc/place.stream.vod.createDraft"
		updateDraft  = "/xrpc/place.stream.vod.updateDraft"
		deleteDraft  = "/xrpc/place.stream.vod.deleteDraft"
		getDraft     = "/xrpc/place.stream.vod.getDraft"
		listDrafts   = "/xrpc/place.stream.vod.listDrafts"
		publishDraft = "/xrpc/place.stream.vod.publishDraft"
		allow        = http.StatusNoContent
		deny         = http.StatusForbidden
	)

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	key, err := jwk.FromRaw(private)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(key)
	require.NoError(t, err)
	op := oatproxy.New(&oatproxy.Config{Host: stationCreatorTestOwner})

	for _, tc := range []struct {
		name, path           string
		downstream, upstream string
		status               int
		bearer               bool
		spaceOnly, unsharded bool
	}{
		{name: "legacy session", path: createTarget, status: allow},
		{name: "full include", path: listTargets, upstream: full, status: allow},
		{name: "requested include without upstream grant", path: listDrafts, downstream: full, status: deny},
		{name: "requested repo without upstream grant", path: createTarget, downstream: target, status: deny},
		{name: "upstream narrowed", path: listDrafts, downstream: full, upstream: "atproto", status: deny},
		{name: "upstream grants only update", path: getDraft, downstream: full, upstream: draft + "&action=update", status: deny},
		{name: "atproto only", path: createTarget, downstream: "atproto", status: deny},
		{name: "nonempty blank scope is not legacy", path: createTarget, downstream: full, upstream: " ", status: deny},
		{name: "other include", path: createTarget, upstream: "include:place.stream.authFullOther", status: deny},
		{name: "audience-bound include", path: createTarget, upstream: "include:place.stream.authFull?aud=did:web:other.example%23service", status: deny},
		{name: "unrelated read", path: listTargets, upstream: "account:email?action=read " + draft, status: deny},
		{name: "repo grant with another authority", path: createTarget, upstream: target + "&authority=did:plc:other", status: deny},
		{name: "unexpanded authority cannot grant read", path: listTargets, upstream: "space:place.stream.personal?authority=self&skey=self&collection=place.stream.multistream.target&action=read", status: deny},
		{name: "default repo grant can list targets", path: listTargets, upstream: "atproto " + target, status: allow},
		{name: "default repo grant can read drafts", path: getDraft, upstream: "atproto " + draft, status: allow},
		{name: "create target", path: createTarget, upstream: target + "&action=create", status: allow},
		{name: "update target", path: putTarget, upstream: "repo:place.stream.multistream.target?action=delete&action=update", status: allow},
		{name: "create cannot update target", path: putTarget, upstream: target + "&action=create", status: deny},
		{name: "delete target", path: deleteTarget, upstream: target + "&action=delete", status: allow},
		{name: "create draft", path: createDraft, upstream: draft + "&action=create", status: allow},
		{name: "update draft", path: updateDraft, upstream: draft + "&action=update", status: allow},
		{name: "update cannot delete draft", path: deleteDraft, upstream: draft + "&action=update", status: deny},
		{name: "delete draft", path: deleteDraft, upstream: draft + "&action=delete", status: allow},
		{name: "publish with full include", path: publishDraft, upstream: full, status: allow},
		{name: "publish with explicit grants", path: publishDraft, upstream: video + "&action=create " + draft + "&action=delete", status: allow},
		{name: "publish without draft grant", path: publishDraft, upstream: video + "&action=create", status: deny},
		{name: "publish without video create", path: publishDraft, downstream: full, upstream: draft + " " + video + "&action=update", status: deny},
		{name: "legacy cannot publish", path: publishDraft, status: deny},
		{name: "method-bound bearer", path: deleteTarget, status: allow, bearer: true},
		{name: "publish requires OAuth", path: publishDraft, status: http.StatusUnauthorized, bearer: true},
		{name: "space target read", path: listTargets, upstream: spaceTarget + "&action=read", status: allow, spaceOnly: true},
		{name: "space target create", path: createTarget, upstream: spaceTarget + "&action=create&action=read", status: allow, spaceOnly: true},
		{name: "space target create without quota read", path: createTarget, upstream: spaceTarget + "&action=create", status: deny},
		{name: "space target update", path: putTarget, upstream: spaceTarget + "&action=update&action=read", status: allow, spaceOnly: true},
		{name: "space target delete", path: deleteTarget, upstream: spaceTarget + "&action=delete", status: allow, spaceOnly: true},
		{name: "space draft get", path: getDraft, upstream: spaceDraft + "&action=read", status: allow, spaceOnly: true},
		{name: "space draft list", path: listDrafts, upstream: spaceDraft + "&action=read_self", status: allow, spaceOnly: true},
		{name: "space draft create", path: createDraft, upstream: spaceDraft + "&action=create", status: allow, spaceOnly: true},
		{name: "space draft update", path: updateDraft, upstream: spaceDraft + "&action=update&action=read", status: allow, spaceOnly: true},
		{name: "space draft update without metadata read", path: updateDraft, upstream: spaceDraft + "&action=update", status: deny},
		{name: "space draft delete", path: deleteDraft, upstream: spaceDraft + "&action=delete", status: allow, spaceOnly: true},
		{name: "space draft publish", path: publishDraft, upstream: spaceDraft + "&action=read " + spaceDraft + "&action=delete " + video + "&action=create", status: allow, spaceOnly: true},
		{name: "both paths authorized", path: deleteTarget, upstream: spaceTarget + "&action=delete " + target + "&action=delete", status: allow},
		{name: "repo permits a different operation", path: deleteTarget, upstream: spaceTarget + "&action=delete " + target + "&action=create", status: allow, spaceOnly: true},
		{name: "space read is not a write grant", path: createTarget, upstream: spaceTarget + "&action=read", status: deny},
		{name: "space write is not a read grant", path: listTargets, upstream: spaceTarget + "&action=update", status: deny},
		{name: "space create is not update", path: updateDraft, upstream: spaceDraft + "&action=create", status: deny},
		{name: "wrong space collection", path: getDraft, upstream: spaceTarget + "&action=read", status: deny},
		{name: "draftVideo is not space metadata", path: getDraft, upstream: boundSpace + "&collection=place.stream.vod.draftVideo&action=read", status: deny},
		{name: "wrong space authority", path: listTargets, upstream: strings.Replace(spaceTarget, did, "did:plc:other", 1) + "&action=read", status: deny},
		{name: "wrong space key", path: listTargets, upstream: strings.Replace(spaceTarget, "skey=self", "skey=other", 1) + "&action=read", status: deny},
		{name: "malformed space grant", path: listTargets, upstream: spaceTarget + "&action=read&action=%", status: deny},
		{name: "requested space is not granted", path: listTargets, downstream: spaceTarget + "&action=read", status: deny},
		{name: "narrowed upstream space", path: listTargets, downstream: spaceTarget + "&action=read", upstream: spaceTarget + "&action=update", status: deny},
		{name: "space publish without draft read", path: publishDraft, upstream: spaceDraft + "&action=delete " + video + "&action=create", status: deny},
		{name: "space publish without draft delete", path: publishDraft, upstream: spaceDraft + "&action=read " + video + "&action=create", status: deny},
		{name: "space publish with only video update", path: publishDraft, upstream: spaceDraft + "&action=read&action=delete " + video + "&action=update", status: deny},
		{name: "space publish with requested video create", path: publishDraft, downstream: video + "&action=create", upstream: spaceDraft + "&action=read&action=delete", status: deny},
		{name: "space video create is not repo video create", path: publishDraft, upstream: spaceDraft + "&action=read&action=delete " + boundSpace + "&collection=place.stream.video&action=create", status: deny},
		{name: "unsharded request bypasses station policy", path: listDrafts, upstream: spaceDraft + "&action=read", status: allow, unsharded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slots := &stationCreatorTestSlots{route: stationCreatorOwnerDID}
			s := &Server{cli: &config.CLI{ServerHost: stationCreatorTestOwner}, creatorRoutes: &creatorRouter{slots: slots}}
			if tc.unsharded {
				s.creatorRoutes = nil
			}
			req := httptest.NewRequest(creatorMethod(tc.path), tc.path+"?spaceOnly=false", strings.NewReader(`{"spaceOnly":false}`)).WithContext(t.Context())
			req.Header.Set("X-Streamplace-Creator-Space-Only", "false")
			ctx := context.WithValue(req.Context(), creatorRouteContextKey{}, creatorRoute{did: did})
			if tc.bearer {
				ctx = context.WithValue(ctx, pdsAuthContextKey, did)
			} else {
				ctx = context.WithValue(ctx, oatproxy.OATProxyContextKey, op)
				ctx = context.WithValue(ctx, oatproxy.OAuthSessionContextKey, &oatproxy.OAuthSession{
					DID: did, UpstreamDPoPPrivateJWK: string(keyJSON),
					DownstreamScope: tc.downstream, UpstreamScope: tc.upstream,
				})
			}
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(req.WithContext(ctx), rec)
			err := s.creatorAuthorize(func(c echo.Context) error {
				require.Equal(t, tc.spaceOnly, creatorSpaceOnly(c.Request().Context()))
				return c.NoContent(allow)
			})(c)
			if err != nil {
				e.HTTPErrorHandler(err, c)
			}
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			_, claims := slots.calls()
			if tc.status == allow && !tc.unsharded {
				require.Equal(t, []string{did}, claims)
			} else {
				require.Empty(t, claims)
			}
		})
	}
}
