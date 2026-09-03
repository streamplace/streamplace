package accessctl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/access"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

const (
	admin    = "did:plc:admin"
	alice    = "did:plc:alice"
	bob      = "did:plc:bob"
	testKey  = "did:key:zTestStream"
	authorit = "did:web:node.example"
)

func newController(t *testing.T, cli *config.CLI) *Controller {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cli.BroadcasterHost = "node.example"
	cli.DBURL = ":memory:"
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(ctx, cli, nil, mod)
	require.NoError(t, err)
	c, err := New(ctx, cli, state, mod)
	require.NoError(t, err)
	return c
}

func TestDefaultsMatchLegacyEnvironmentSemantics(t *testing.T) {
	ctx := context.Background()

	t.Run("fresh node is open for viewing and streaming, closed for syndication", func(t *testing.T) {
		c := newController(t, &config.CLI{})
		m := c.Modes()
		require.Equal(t, access.ModeOpen, m[access.RoleViewer])
		require.Equal(t, access.ModeOpen, m[access.RoleStreamer])
		require.Equal(t, access.ModeOff, m[access.RoleSyndicate])
		require.Equal(t, access.ModeAllowlist, m[access.RoleAdmin])
		require.True(t, c.Allowed(ctx, "", access.RoleViewer), "anonymous viewer on an open node")
		require.True(t, c.Allowed(ctx, alice, access.RoleStreamer))
		require.False(t, c.Allowed(ctx, testKey, access.RoleStreamer), "did:key never gets the open-server benefit")
		require.False(t, c.Allowed(ctx, alice, access.RoleSyndicate))
		require.False(t, c.Allowed(ctx, alice, access.RoleAdmin))
		// legacy fallback: streamers may upload when no invite issuer is set
		require.True(t, c.Allowed(ctx, alice, access.RoleVOD))
	})

	t.Run("SP_ALLOWED_STREAMS seeds an allowlist", func(t *testing.T) {
		c := newController(t, &config.CLI{AllowedStreams: []string{alice, testKey}})
		require.Equal(t, access.ModeAllowlist, c.Modes()[access.RoleStreamer])
		require.True(t, c.Allowed(ctx, alice, access.RoleStreamer))
		require.True(t, c.Allowed(ctx, testKey, access.RoleStreamer))
		require.False(t, c.Allowed(ctx, bob, access.RoleStreamer))
		require.False(t, c.Allowed(ctx, bob, access.RoleVOD), "vod follows the streamer allowlist by default")
	})

	t.Run("SP_SYNDICATE list and wildcard", func(t *testing.T) {
		c := newController(t, &config.CLI{Syndicate: []string{alice}})
		require.Equal(t, access.ModeAllowlist, c.Modes()[access.RoleSyndicate])
		require.True(t, c.Allowed(ctx, alice, access.RoleSyndicate))
		require.False(t, c.Allowed(ctx, bob, access.RoleSyndicate))

		c = newController(t, &config.CLI{Syndicate: []string{"*"}})
		require.Equal(t, access.ModeOpen, c.Modes()[access.RoleSyndicate])
		require.True(t, c.Allowed(ctx, bob, access.RoleSyndicate))

		c = newController(t, &config.CLI{Syndicate: []string{"*"}, DisableSyndication: true})
		require.Equal(t, access.ModeOff, c.Modes()[access.RoleSyndicate])
		require.False(t, c.Allowed(ctx, bob, access.RoleSyndicate))
	})

	t.Run("SP_WIDE_OPEN opens everything but admin", func(t *testing.T) {
		c := newController(t, &config.CLI{WideOpen: true, AllowedStreams: []string{alice}})
		require.True(t, c.Allowed(ctx, bob, access.RoleStreamer))
		require.True(t, c.Allowed(ctx, bob, access.RoleVOD))
		require.False(t, c.Allowed(ctx, bob, access.RoleAdmin))
	})

	t.Run("SP_ADMIN_DIDS implies every role", func(t *testing.T) {
		c := newController(t, &config.CLI{AdminDIDs: []string{admin}, Syndicate: nil})
		for _, role := range access.Roles {
			require.True(t, c.Allowed(ctx, admin, role), role)
		}
	})

	t.Run("SP_ACCESS_POLICY seeds modes and the record overrides it", func(t *testing.T) {
		c := newController(t, &config.CLI{
			AdminDIDs:    []string{admin},
			AccessPolicy: map[string]string{access.RoleViewer: access.ModeAllowlist, access.RoleVOD: access.ModeAllowlist},
		})
		require.Equal(t, access.ModeAllowlist, c.Modes()[access.RoleViewer])
		require.False(t, c.Allowed(ctx, "", access.RoleViewer), "private from first boot")
		require.False(t, c.Allowed(ctx, alice, access.RoleViewer))
		require.True(t, c.Allowed(ctx, admin, access.RoleViewer), "the seeded admin gets in")
		require.False(t, c.Allowed(ctx, alice, access.RoleVOD), "a seeded vod mode is explicit: no streamer fallback")
		require.NoError(t, c.UpdatePolicy(ctx, []placestream.AccessDefs_RoleMode{{Role: access.RoleViewer, Mode: access.ModeOpen}}))
		require.Equal(t, access.ModeOpen, c.Modes()[access.RoleViewer], "the app's setting wins over the seed")
	})

	t.Run("SP_DISABLE_SYNDICATION is a kill switch even for admins", func(t *testing.T) {
		c := newController(t, &config.CLI{AdminDIDs: []string{admin}, Syndicate: []string{"*"}, DisableSyndication: true})
		require.False(t, c.Allowed(ctx, admin, access.RoleSyndicate))
		require.True(t, c.Allowed(ctx, admin, access.RoleViewer))
	})
}

func TestPolicyAndGrants(t *testing.T) {
	ctx := context.Background()
	c := newController(t, &config.CLI{AdminDIDs: []string{admin}})

	// Lock the node down.
	require.NoError(t, c.UpdatePolicy(ctx, []placestream.AccessDefs_RoleMode{{Role: access.RoleViewer, Mode: access.ModeAllowlist}}))
	require.Equal(t, access.ModeAllowlist, c.Modes()[access.RoleViewer])
	require.False(t, c.Allowed(ctx, "", access.RoleViewer))
	require.False(t, c.Allowed(ctx, alice, access.RoleViewer))
	require.True(t, c.Allowed(ctx, admin, access.RoleViewer), "admins always pass")

	// Grant alice, twice: idempotent.
	g1, err := c.CreateGrant(ctx, admin, alice, access.RoleViewer, nil)
	require.NoError(t, err)
	require.NotNil(t, g1.Uri)
	ref, err := access.ParseGrantURI(*g1.Uri)
	require.NoError(t, err)
	require.Equal(t, authorit, ref.Authority)
	require.Equal(t, admin, ref.Author)
	g2, err := c.CreateGrant(ctx, admin, alice, access.RoleViewer, nil)
	require.NoError(t, err)
	require.Equal(t, *g1.Uri, *g2.Uri)
	require.True(t, c.Allowed(ctx, alice, access.RoleViewer))
	require.False(t, c.Allowed(ctx, alice, access.RoleStreamer) == false, "streamer stays open")

	// Listing shows the env admin and the space grant with their sources.
	grants, err := c.ListGrants(ctx, "")
	require.NoError(t, err)
	require.Len(t, grants, 2)
	require.Equal(t, access.SourceEnvironment, grants[0].Source)
	require.Nil(t, grants[0].Uri)
	require.Equal(t, access.SourceSpace, grants[1].Source)
	require.Equal(t, alice, grants[1].Subject)
	only, err := c.ListGrants(ctx, access.RoleViewer)
	require.NoError(t, err)
	require.Len(t, only, 1)

	// Revoke.
	require.NoError(t, c.DeleteGrant(ctx, *g1.Uri))
	require.False(t, c.Allowed(ctx, alice, access.RoleViewer))
	require.ErrorIs(t, c.DeleteGrant(ctx, *g1.Uri), access.ErrNotFound)
	require.ErrorIs(t, c.DeleteGrant(ctx, access.GrantURI("did:web:other", admin, "x")), access.ErrNotFound)
	require.Error(t, c.DeleteGrant(ctx, "at://did:web:x/place.stream.access.grant/3abc"))

	// Policy edits are partial merges and admin is immutable.
	require.NoError(t, c.UpdatePolicy(ctx, []placestream.AccessDefs_RoleMode{{Role: access.RoleVOD, Mode: access.ModeOff}}))
	m := c.Modes()
	require.Equal(t, access.ModeAllowlist, m[access.RoleViewer], "earlier setting kept")
	require.Equal(t, access.ModeOff, m[access.RoleVOD])
	require.False(t, c.Allowed(ctx, alice, access.RoleVOD))
	require.Error(t, c.UpdatePolicy(ctx, []placestream.AccessDefs_RoleMode{{Role: access.RoleAdmin, Mode: access.ModeOpen}}))
	require.Error(t, c.UpdatePolicy(ctx, []placestream.AccessDefs_RoleMode{{Role: access.RoleVOD, Mode: "sometimes"}}))

	// An explicit vod allowlist no longer inherits from streamer.
	require.NoError(t, c.UpdatePolicy(ctx, []placestream.AccessDefs_RoleMode{{Role: access.RoleVOD, Mode: access.ModeAllowlist}}))
	require.False(t, c.Allowed(ctx, alice, access.RoleVOD))
	_, err = c.CreateGrant(ctx, admin, alice, access.RoleVOD, nil)
	require.NoError(t, err)
	require.True(t, c.Allowed(ctx, alice, access.RoleVOD))

	// Space admin grants work like env admins.
	_, err = c.CreateGrant(ctx, admin, bob, access.RoleAdmin, nil)
	require.NoError(t, err)
	require.True(t, c.Allowed(ctx, bob, access.RoleAdmin))
	require.True(t, c.Allowed(ctx, bob, access.RoleViewer))

	// Bad input.
	_, err = c.CreateGrant(ctx, admin, "alice.example", access.RoleViewer, nil)
	require.Error(t, err, "handles must be resolved before reaching the controller")
	_, err = c.CreateGrant(ctx, admin, alice, "wizard", nil)
	require.Error(t, err)
}

func TestSnapshotSurvivesReloadFromDB(t *testing.T) {
	ctx := context.Background()
	c := newController(t, &config.CLI{AdminDIDs: []string{admin}})
	require.NoError(t, c.UpdatePolicy(ctx, []placestream.AccessDefs_RoleMode{{Role: access.RoleViewer, Mode: access.ModeAllowlist}}))
	_, err := c.CreateGrant(ctx, admin, alice, access.RoleViewer, nil)
	require.NoError(t, err)

	// A second controller over the same statedb (what another node in the
	// station sees after its refresh tick) reads the same answer.
	c2, err := New(ctx, c.cli, c.state, c.mod)
	require.NoError(t, err)
	require.Equal(t, access.ModeAllowlist, c2.Modes()[access.RoleViewer])
	require.True(t, c2.Allowed(ctx, alice, access.RoleViewer))
}

func TestViewerCookie(t *testing.T) {
	c := newController(t, &config.CLI{})
	ck := c.ViewerCookie(alice)
	require.Equal(t, ViewerCookieName, ck.Name)
	require.True(t, ck.HttpOnly)

	r := httptest.NewRequest(http.MethodGet, "/api/playback/x/stream.jpg", nil)
	r.AddCookie(ck)
	did, ok := c.ViewerFromCookie(r)
	require.True(t, ok)
	require.Equal(t, alice, did)

	// tampering, expiry, and a different key all fail
	bad := httptest.NewRequest(http.MethodGet, "/", nil)
	bad.AddCookie(&http.Cookie{Name: ViewerCookieName, Value: ck.Value + "x"})
	_, ok = c.ViewerFromCookie(bad)
	require.False(t, ok)
	_, ok = c.verify(ck.Value, time.Now().Add(8*24*time.Hour))
	require.False(t, ok)
	other := newController(t, &config.CLI{})
	_, ok = other.verify(ck.Value, time.Now())
	require.False(t, ok)
	none := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok = c.ViewerFromCookie(none)
	require.False(t, ok)
}
