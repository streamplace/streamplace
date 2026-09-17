// Package accessctl implements access.Manager on top of statedb: it merges
// the node's environment-seeded lists (SP_ADMIN_DIDS, SP_ALLOWED_STREAMS,
// SP_SYNDICATE) with the grants and policy stored in the node's
// access-control space, and answers every gate from an in-memory snapshot.
package accessctl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"stream.place/streamplace/pkg/access"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// refreshInterval bounds how stale a node's snapshot can be when another
// node sharing the same statedb edits the policy.
const refreshInterval = 30 * time.Second

// Controller is the node's access.Manager.
type Controller struct {
	cli   *config.CLI
	state *statedb.StatefulDB
	mod   model.Model

	cookieKey []byte

	mu sync.RWMutex
	// grants: role -> subject DID -> grant row.
	grants map[string]map[string]statedb.AccessGrant
	// policy: role -> mode, only for roles the policy record sets explicitly.
	policy map[string]string
}

var _ access.Manager = (*Controller)(nil)

// New loads the snapshot and starts a background refresh that stops with ctx.
func New(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB, mod model.Model) (*Controller, error) {
	key, err := state.EnsureAccessCookieKey(ctx)
	if err != nil {
		return nil, err
	}
	c := &Controller{
		cli:       cli,
		state:     state,
		mod:       mod,
		cookieKey: key,
		grants:    map[string]map[string]statedb.AccessGrant{},
		policy:    map[string]string{},
	}
	if err := c.Reload(ctx); err != nil {
		return nil, err
	}
	go c.refreshLoop(ctx)
	return c, nil
}

func (c *Controller) authority() string {
	return c.cli.BroadcasterDID()
}

// Reload replaces the snapshot from statedb.
func (c *Controller) Reload(ctx context.Context) error {
	rows, err := c.state.ListAccessGrants(ctx, c.authority())
	if err != nil {
		return err
	}
	grants := map[string]map[string]statedb.AccessGrant{}
	for _, g := range rows {
		if grants[g.Role] == nil {
			grants[g.Role] = map[string]statedb.AccessGrant{}
		}
		// Oldest grant wins for a duplicate (subject, role); ListAccessGrants
		// is ordered oldest first.
		if _, dup := grants[g.Role][g.SubjectDID]; !dup {
			grants[g.Role][g.SubjectDID] = g
		}
	}
	policy := map[string]string{}
	rec, err := c.state.GetAccessPolicy(ctx, c.authority())
	if err != nil {
		return err
	}
	if rec != nil {
		for _, rm := range rec.Roles {
			if access.ValidRole(rm.Role) && access.ValidMode(rm.Mode) && rm.Role != access.RoleAdmin {
				policy[rm.Role] = rm.Mode
			}
		}
	}
	c.mu.Lock()
	c.grants = grants
	c.policy = policy
	c.mu.Unlock()
	return nil
}

func (c *Controller) refreshLoop(ctx context.Context) {
	t := time.NewTicker(refreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.Reload(ctx); err != nil && ctx.Err() == nil {
				log.Error(ctx, "access: failed to refresh snapshot", "error", err)
			}
		}
	}
}

// Modes returns the effective mode of every role: the policy record when it
// sets the role, else SP_ACCESS_POLICY, else the default.
//
// Defaults preserve what the environment flags meant before the policy
// record existed: no SP_ALLOWED_STREAMS means anyone may stream, an empty
// SP_SYNDICATE means nothing is syndicated, "*" means everything is, and
// SP_WIDE_OPEN / SP_DISABLE_SYNDICATION still win over everything.
func (c *Controller) Modes() map[string]string {
	c.mu.RLock()
	policy := c.policy
	c.mu.RUnlock()

	modes := map[string]string{access.RoleAdmin: access.ModeAllowlist}

	pick := func(role, def string) string {
		if m, ok := policy[role]; ok {
			return m
		}
		if m, ok := c.cli.AccessPolicy[role]; ok {
			return m
		}
		return def
	}

	streamerDefault := access.ModeAllowlist
	if c.openStreamServer() {
		streamerDefault = access.ModeOpen
	}
	syndicateDefault := access.ModeOff
	if len(c.cli.Syndicate) > 0 {
		syndicateDefault = access.ModeAllowlist
		for _, d := range c.cli.Syndicate {
			if d == "*" {
				syndicateDefault = access.ModeOpen
			}
		}
	}

	modes[access.RoleViewer] = pick(access.RoleViewer, access.ModeOpen)
	modes[access.RoleStreamer] = pick(access.RoleStreamer, streamerDefault)
	modes[access.RoleSyndicate] = pick(access.RoleSyndicate, syndicateDefault)
	modes[access.RoleVOD] = pick(access.RoleVOD, access.ModeAllowlist)

	if c.cli.WideOpen {
		modes[access.RoleViewer] = access.ModeOpen
		modes[access.RoleStreamer] = access.ModeOpen
		modes[access.RoleVOD] = access.ModeOpen
	}
	if c.cli.DisableSyndication {
		modes[access.RoleSyndicate] = access.ModeOff
	}
	return modes
}

// openStreamServer mirrors the historical StreamIsAllowed fallback: with no
// allowlist configured (or only the auto-generated test stream) anyone with
// a real atproto account may stream.
func (c *Controller) openStreamServer() bool {
	return len(c.cli.AllowedStreams) == 0 || (c.cli.TestStream && len(c.cli.AllowedStreams) == 1)
}

// Allowed implements access.Checker.
func (c *Controller) Allowed(ctx context.Context, did, role string) bool {
	if !access.ValidRole(role) {
		return false
	}
	if role == access.RoleAdmin {
		return did != "" && c.isAdmin(did)
	}
	// SP_DISABLE_SYNDICATION means "off in both directions" for everyone;
	// it is an operator kill switch, not a role, so admins do not bypass it.
	if role == access.RoleSyndicate && c.cli.DisableSyndication {
		return false
	}
	if did != "" && c.isAdmin(did) {
		return true
	}
	switch c.Modes()[role] {
	case access.ModeOff:
		return false
	case access.ModeOpen:
		// did:key identities only exist for local test streams; they never
		// get the open-server benefit of the doubt and must be listed.
		if role == access.RoleStreamer && strings.HasPrefix(did, constants.DID_KEY_PREFIX) {
			return c.granted(did, role)
		}
		return true
	case access.ModeAllowlist:
		if did == "" {
			return false
		}
		if c.granted(did, role) {
			return true
		}
		if role == access.RoleVOD {
			return c.vodFallback(ctx, did)
		}
		return false
	}
	return false
}

func (c *Controller) isAdmin(did string) bool {
	return c.granted(did, access.RoleAdmin)
}

// granted reports an explicit grant of role to did from either source.
func (c *Controller) granted(did, role string) bool {
	if c.envGranted(did, role) {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.grants[role][did]
	return ok
}

// envGranted reports a grant seeded from the node's environment.
func (c *Controller) envGranted(did, role string) bool {
	var list []string
	switch role {
	case access.RoleAdmin:
		list = c.cli.AdminDIDs
	case access.RoleStreamer:
		list = c.cli.AllowedStreams
	case access.RoleSyndicate:
		list = c.cli.Syndicate
	default:
		return false
	}
	for _, d := range list {
		if d != "*" && d == did {
			return true
		}
	}
	return false
}

// vodFallback keeps the beta program and the pre-RBAC behaviour working:
// a trusted place.stream.beta.invite still grants vod, and when no invite
// issuer is configured and the operator has not set a vod mode explicitly,
// anyone who may stream may also upload (what the old code did).
func (c *Controller) vodFallback(ctx context.Context, did string) bool {
	if c.cli.BetaInviteDID != "" {
		if c.mod == nil {
			return false
		}
		has, err := c.mod.HasBetaInvite(ctx, c.cli.BetaInviteDID, did, "vod")
		if err != nil {
			log.Error(ctx, "access: beta invite lookup failed", "error", err, "did", did)
			return false
		}
		return has
	}
	c.mu.RLock()
	_, explicit := c.policy[access.RoleVOD]
	c.mu.RUnlock()
	if _, seeded := c.cli.AccessPolicy[access.RoleVOD]; explicit || seeded {
		return false
	}
	return c.Allowed(ctx, did, access.RoleStreamer)
}

// ListGrants implements access.Manager.
func (c *Controller) ListGrants(ctx context.Context, role string) ([]placestream.AccessDefs_GrantView, error) {
	if role != "" && !access.ValidRole(role) {
		return nil, fmt.Errorf("unknown role %q", role)
	}
	var out []placestream.AccessDefs_GrantView
	seed := func(r string, dids []string) {
		if role != "" && role != r {
			return
		}
		for _, d := range dids {
			if d == "*" || d == "" {
				continue
			}
			out = append(out, placestream.AccessDefs_GrantView{
				Subject: d,
				Role:    r,
				Source:  access.SourceEnvironment,
			})
		}
	}
	seed(access.RoleAdmin, c.cli.AdminDIDs)
	seed(access.RoleStreamer, c.cli.AllowedStreams)
	seed(access.RoleSyndicate, c.cli.Syndicate)

	rows, err := c.state.ListAccessGrants(ctx, c.authority())
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if role != "" && rows[i].Role != role {
			continue
		}
		out = append(out, grantView(&rows[i]))
	}
	if out == nil {
		out = []placestream.AccessDefs_GrantView{}
	}
	return out, nil
}

func grantView(g *statedb.AccessGrant) placestream.AccessDefs_GrantView {
	uri, cid, author := g.URI, g.CID, g.AuthorDID
	v := placestream.AccessDefs_GrantView{
		Uri:       &uri,
		Cid:       &cid,
		Subject:   g.SubjectDID,
		Role:      g.Role,
		Source:    access.SourceSpace,
		CreatedBy: &author,
	}
	if rec, err := g.Record(); err == nil {
		createdAt := rec.CreatedAt
		v.CreatedAt = &createdAt
		v.Note = rec.Note
	}
	return v
}

// CreateGrant implements access.Manager.
func (c *Controller) CreateGrant(ctx context.Context, author, subject, role string, note *string) (*placestream.AccessDefs_GrantView, error) {
	if !access.ValidRole(role) {
		return nil, fmt.Errorf("unknown role %q", role)
	}
	if !strings.HasPrefix(subject, "did:") {
		return nil, fmt.Errorf("subject must be a DID, got %q", subject)
	}
	existing, err := c.state.FindAccessGrant(ctx, c.authority(), subject, role)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		v := grantView(existing)
		return &v, nil
	}
	rec := &placestream.AccessGrant{
		Subject:   subject,
		Role:      role,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Note:      note,
	}
	g, err := c.state.CreateAccessGrant(ctx, c.authority(), author, rec)
	if err != nil {
		return nil, err
	}
	if err := c.Reload(ctx); err != nil {
		return nil, err
	}
	v := grantView(g)
	return &v, nil
}

// DeleteGrant implements access.Manager.
func (c *Controller) DeleteGrant(ctx context.Context, uri string) error {
	ref, err := access.ParseGrantURI(uri)
	if err != nil {
		return err
	}
	if ref.Authority != c.authority() {
		return access.ErrNotFound
	}
	if err := c.state.DeleteAccessGrant(ctx, uri); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return access.ErrNotFound
		}
		return err
	}
	return c.Reload(ctx)
}

// UpdatePolicy implements access.Manager.
func (c *Controller) UpdatePolicy(ctx context.Context, roles []placestream.AccessDefs_RoleMode) error {
	for _, rm := range roles {
		if rm.Role == access.RoleAdmin {
			return fmt.Errorf("the admin role is always %s", access.ModeAllowlist)
		}
		if !access.ValidRole(rm.Role) {
			return fmt.Errorf("unknown role %q", rm.Role)
		}
		if !access.ValidMode(rm.Mode) {
			return fmt.Errorf("unknown mode %q", rm.Mode)
		}
	}
	current, err := c.state.GetAccessPolicy(ctx, c.authority())
	if err != nil {
		return err
	}
	merged := map[string]string{}
	if current != nil {
		for _, rm := range current.Roles {
			merged[rm.Role] = rm.Mode
		}
	}
	for _, rm := range roles {
		merged[rm.Role] = rm.Mode
	}
	rec := &placestream.AccessPolicy{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, role := range access.Roles {
		if m, ok := merged[role]; ok {
			rec.Roles = append(rec.Roles, placestream.AccessDefs_RoleMode{Role: role, Mode: m})
		}
	}
	if rec.Roles == nil {
		rec.Roles = []placestream.AccessDefs_RoleMode{}
	}
	if err := c.state.PutAccessPolicy(ctx, c.authority(), rec); err != nil {
		return err
	}
	return c.Reload(ctx)
}
