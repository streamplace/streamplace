// Package access defines the node's role-based access control vocabulary:
// the roles a node can grant, the modes that decide each role, and the
// addressing of the grants themselves.
//
// Grants are modeled as records in an atproto space (the "permissioned
// data" spec, bluesky-social/proposals 0016). The node's access-control
// space has the broadcaster DID as its authority, space type
// place.stream.access.control and skey "self"; a grant is a
// place.stream.access.grant record authored by the admin who created it,
// and the policy is a single place.stream.access.policy record authored by
// the authority. Until the spaces implementation ships, the node keeps
// those records in statedb addressed by their at:// space URIs, so moving
// them into a real space later is a data migration and not a redesign.
//
// This package is a leaf: pkg/config depends on it, so it must not import
// anything above pkg/placestream.
package access

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"stream.place/streamplace/pkg/placestream"
)

// Roles a node can grant. Admin implies every other role.
const (
	RoleAdmin     = "admin"
	RoleViewer    = "viewer"
	RoleStreamer  = "streamer"
	RoleSyndicate = "syndicate"
	RoleVOD       = "vod"
)

// Roles lists every role in display order.
var Roles = []string{RoleAdmin, RoleViewer, RoleStreamer, RoleSyndicate, RoleVOD}

// Modes decide how a role is held.
const (
	// ModeOpen: every account (including anonymous visitors) holds the role.
	ModeOpen = "open"
	// ModeAllowlist: only accounts with a grant hold the role.
	ModeAllowlist = "allowlist"
	// ModeOff: nobody holds the role. Admins are always exempt.
	ModeOff = "off"
)

// Grant sources, as reported in place.stream.access.defs#grantView.
const (
	SourceSpace       = "space"
	SourceEnvironment = "environment"
)

// Space addressing.
const (
	SpaceType        = "place.stream.access.control"
	SpaceKey         = "self"
	GrantCollection  = "place.stream.access.grant"
	PolicyCollection = "place.stream.access.policy"
	PolicyRKey       = "self"
)

// ValidRole reports whether r is a role this node knows about.
func ValidRole(r string) bool {
	for _, known := range Roles {
		if known == r {
			return true
		}
	}
	return false
}

// ValidMode reports whether m is a mode this node knows about.
func ValidMode(m string) bool {
	return m == ModeOpen || m == ModeAllowlist || m == ModeOff
}

// ParsePolicy parses "role=mode,role=mode" (the --access-policy flag).
// The admin role cannot be set; it is always allowlist.
func ParsePolicy(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		role, mode, ok := strings.Cut(pair, "=")
		role, mode = strings.TrimSpace(role), strings.TrimSpace(mode)
		if !ok || !ValidRole(role) || !ValidMode(mode) {
			return nil, fmt.Errorf("expected role=mode with role in %v and mode in [%s %s %s], got %q", Roles, ModeOpen, ModeAllowlist, ModeOff, pair)
		}
		if role == RoleAdmin {
			return nil, fmt.Errorf("the admin role is always %s", ModeAllowlist)
		}
		out[role] = mode
	}
	return out, nil
}

// SpaceURI is the node's access-control space:
// at://{authority}/space/place.stream.access.control/self
func SpaceURI(authority string) string {
	return fmt.Sprintf("at://%s/space/%s/%s", authority, SpaceType, SpaceKey)
}

// GrantURI addresses one grant record inside the space:
// at://{authority}/space/place.stream.access.control/self/{author}/place.stream.access.grant/{rkey}
func GrantURI(authority, author, rkey string) string {
	return fmt.Sprintf("%s/%s/%s/%s", SpaceURI(authority), author, GrantCollection, rkey)
}

// PolicyURI addresses the node's single policy record, authored by the
// authority itself.
func PolicyURI(authority string) string {
	return fmt.Sprintf("%s/%s/%s/%s", SpaceURI(authority), authority, PolicyCollection, PolicyRKey)
}

// GrantRef is a parsed grant URI.
type GrantRef struct {
	Authority string
	Author    string
	RKey      string
}

// ParseGrantURI validates that uri has the grant shape for this space type
// and returns its parts.
func ParseGrantURI(uri string) (GrantRef, error) {
	rest, ok := strings.CutPrefix(uri, "at://")
	if !ok {
		return GrantRef{}, fmt.Errorf("not an at:// URI: %q", uri)
	}
	parts := strings.Split(rest, "/")
	// authority, "space", type, skey, author, collection, rkey
	if len(parts) != 7 || parts[1] != "space" || parts[2] != SpaceType || parts[3] != SpaceKey || parts[5] != GrantCollection {
		return GrantRef{}, fmt.Errorf("not a %s grant URI: %q", SpaceType, uri)
	}
	for _, seg := range []string{parts[0], parts[4], parts[6]} {
		if seg == "" {
			return GrantRef{}, fmt.Errorf("empty segment in grant URI: %q", uri)
		}
	}
	return GrantRef{Authority: parts[0], Author: parts[4], RKey: parts[6]}, nil
}

// Checker answers the one question every gate in the node asks.
type Checker interface {
	// Allowed reports whether did holds role. did may be empty for an
	// anonymous caller; only open-mode roles are held anonymously.
	Allowed(ctx context.Context, did, role string) bool
	// Modes returns the effective mode of every role, after environment
	// overrides such as SP_WIDE_OPEN and SP_DISABLE_SYNDICATION.
	Modes() map[string]string
}

// Manager is a Checker that can also be administered.
type Manager interface {
	Checker
	// ListGrants returns every grant, or only those of role when non-empty.
	// Environment-seeded grants are included with SourceEnvironment.
	ListGrants(ctx context.Context, role string) ([]placestream.AccessDefs_GrantView, error)
	// CreateGrant grants role to subject (a DID) on behalf of author. It is
	// idempotent: an existing grant is returned unchanged.
	CreateGrant(ctx context.Context, author, subject, role string, note *string) (*placestream.AccessDefs_GrantView, error)
	// DeleteGrant revokes the grant at uri. Returns ErrNotFound when there
	// is no such grant.
	DeleteGrant(ctx context.Context, uri string) error
	// UpdatePolicy sets the mode of the given roles, keeping the rest.
	UpdatePolicy(ctx context.Context, roles []placestream.AccessDefs_RoleMode) error
	// ViewerCookie mints the cookie that lets non-XRPC routes (playback,
	// chat websockets, thumbnails) recognise an allowed viewer.
	ViewerCookie(did string) *http.Cookie
	// ViewerFromCookie returns the DID a request's viewer cookie vouches for.
	ViewerFromCookie(r *http.Request) (string, bool)
}

// ErrNotFound is returned by DeleteGrant when the URI matches nothing.
var ErrNotFound = fmt.Errorf("grant not found")
