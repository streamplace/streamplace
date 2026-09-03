package spxrpc

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"

	"stream.place/streamplace/pkg/access"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// accessGateExempt lists the XRPC methods a private node still answers to
// accounts without the viewer role: what a client needs to boot, discover
// the node, learn that it is locked out, and complete a login.
var accessGateExempt = []string{
	"/xrpc/_health",
	"/xrpc/place.stream.access.getStatus",
	"/xrpc/place.stream.broadcast.getBroadcaster",
	"/xrpc/place.stream.config.getEnv",
	// Branding is public so the sign-in wall can carry the node's own logo
	// and name; getBlob serves the logo bytes getBranding points at.
	"/xrpc/place.stream.branding.getBranding",
	"/xrpc/place.stream.branding.getBlob",
	// The client syncs its clock against the node before OAuth (DPoP proofs
	// carry timestamps), so this has to answer before anyone can sign in.
	"/xrpc/place.stream.server.getServerTime",
	"/xrpc/com.atproto.identity.",
	"/xrpc/com.atproto.server.",
	// Profile lookups are proxied upstream to the caller's own PDS / the
	// AppView and reveal nothing about this node. The client fetches its
	// own profile right after login (and treats a failure as "logged out"),
	// and the sign-in wall wants handle autocomplete and a display name.
	"/xrpc/app.bsky.actor.getProfile",
	"/xrpc/app.bsky.actor.getProfiles",
	"/xrpc/app.bsky.actor.searchActorsTypeahead",
}

func accessGateExempted(path string) bool {
	for _, p := range accessGateExempt {
		if strings.HasSuffix(p, ".") {
			if strings.HasPrefix(path, p) {
				return true
			}
		} else if path == p {
			return true
		}
	}
	return false
}

// AccessGateMiddleware enforces the viewer role on every XRPC call when the
// node's viewer mode is not open. It runs after OAuthMiddleware so the
// caller's DID is known. Peer nodes authenticated with the shared service
// key pass: they are the operator's own infrastructure.
//
// Identity comes from the DPoP session when the call carries one, and
// otherwise from the viewer cookie. The client makes most of its reads
// (live users, recommendations, segments, playback) through an anonymous
// agent even when signed in, and the browser attaches the cookie to those
// same-origin fetches; the cookie is minted here whenever a DPoP-authenticated
// call from an allowed viewer comes through, which the client's status fetch
// after sign-in guarantees. The cookie only satisfies this gate: handlers
// that need to know who the caller is still require the DPoP session.
func (s *Server) AccessGateMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ac := s.cli.Access
			if ac == nil || ac.Modes()[access.RoleViewer] == access.ModeOpen {
				return next(c)
			}
			ctx := c.Request().Context()
			if GetServiceAuth(ctx) != nil {
				return next(c)
			}
			session, _ := oatproxy.GetOAuthSession(ctx)
			var did string
			if session != nil {
				did = session.DID
			}
			allowed := did != "" && ac.Allowed(ctx, did, access.RoleViewer)
			if allowed {
				c.SetCookie(ac.ViewerCookie(did))
			} else if did == "" {
				if cookieDID, ok := ac.ViewerFromCookie(c.Request()); ok {
					did = cookieDID
					allowed = ac.Allowed(ctx, did, access.RoleViewer)
				}
			}
			if accessGateExempted(c.Request().URL.Path) || allowed {
				return next(c)
			}
			if did == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "AuthRequired: this node is private; sign in to continue")
			}
			return echo.NewHTTPError(http.StatusForbidden, "ViewerRequired: this account is not on this node's viewer list")
		}
	}
}

// requireAdmin returns the caller's DID or an error suitable for returning
// from a handler.
func (s *Server) requireAdmin(ctx context.Context, what string) (string, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	if !s.cli.IsAdmin(session.DID) {
		log.Warn(ctx, "unauthorized admin attempt", "did", session.DID, "what", what)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized: not authorized to "+what)
	}
	return session.DID, nil
}

func (s *Server) accessManager() (access.Manager, error) {
	if s.cli.Access == nil {
		return nil, echo.NewHTTPError(http.StatusNotImplemented, "access control is not configured on this node")
	}
	return s.cli.Access, nil
}

func policyView(ac access.Checker) placestream.AccessDefs_PolicyView {
	modes := ac.Modes()
	out := placestream.AccessDefs_PolicyView{Roles: make([]placestream.AccessDefs_RoleMode, 0, len(access.Roles))}
	for _, role := range access.Roles {
		out.Roles = append(out.Roles, placestream.AccessDefs_RoleMode{Role: role, Mode: modes[role]})
	}
	return out
}

func (s *Server) handlePlaceStreamAccessGetStatus(ctx context.Context) (*placestream.AccessGetStatus_Output, error) {
	ac, err := s.accessManager()
	if err != nil {
		return nil, err
	}
	out := &placestream.AccessGetStatus_Output{
		Roles:  []string{},
		Policy: policyView(ac),
		Space:  access.SpaceURI(s.cli.BroadcasterDID()),
	}
	var did string
	if session, _ := oatproxy.GetOAuthSession(ctx); session != nil {
		did = session.DID
		out.Did = &did
	}
	for _, role := range access.Roles {
		if ac.Allowed(ctx, did, role) {
			out.Roles = append(out.Roles, role)
		}
	}
	return out, nil
}

func (s *Server) handlePlaceStreamAccessListGrants(ctx context.Context, role string) (*placestream.AccessListGrants_Output, error) {
	ac, err := s.accessManager()
	if err != nil {
		return nil, err
	}
	if _, err := s.requireAdmin(ctx, "list grants"); err != nil {
		return nil, err
	}
	if role != "" && !access.ValidRole(role) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidRole: unknown role "+role)
	}
	grants, err := ac.ListGrants(ctx, role)
	if err != nil {
		log.Error(ctx, "failed to list access grants", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to list grants")
	}
	return &placestream.AccessListGrants_Output{Grants: grants}, nil
}

// resolveSubject turns a DID or handle into a DID.
func (s *Server) resolveSubject(ctx context.Context, subject string) (string, error) {
	subject = strings.TrimSpace(strings.TrimPrefix(subject, "@"))
	if subject == "" {
		return "", errors.New("empty subject")
	}
	if strings.HasPrefix(subject, "did:") {
		if _, err := syntax.ParseDID(subject); err != nil {
			return "", err
		}
		return subject, nil
	}
	if _, err := syntax.ParseHandle(subject); err != nil {
		return "", err
	}
	if s.ATSync == nil {
		return "", errors.New("handle resolution unavailable")
	}
	repo, err := s.ATSync.SyncBlueskyRepoCached(ctx, subject)
	if err != nil {
		return "", err
	}
	if repo == nil || repo.DID == "" {
		return "", errors.New("handle did not resolve")
	}
	return repo.DID, nil
}

func (s *Server) handlePlaceStreamAccessCreateGrant(ctx context.Context, input *placestream.AccessCreateGrant_Input) (*placestream.AccessCreateGrant_Output, error) {
	ac, err := s.accessManager()
	if err != nil {
		return nil, err
	}
	author, err := s.requireAdmin(ctx, "create grants")
	if err != nil {
		return nil, err
	}
	if !access.ValidRole(input.Role) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidRole: unknown role "+input.Role)
	}
	subject, err := s.resolveSubject(ctx, input.Subject)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidSubject: "+err.Error())
	}
	grant, err := ac.CreateGrant(ctx, author, subject, input.Role, input.Note)
	if err != nil {
		log.Error(ctx, "failed to create access grant", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to create grant")
	}
	log.Log(ctx, "access grant created", "subject", subject, "role", input.Role, "by", author)
	return &placestream.AccessCreateGrant_Output{Grant: *grant}, nil
}

func (s *Server) handlePlaceStreamAccessDeleteGrant(ctx context.Context, input *placestream.AccessDeleteGrant_Input) (*placestream.AccessDeleteGrant_Output, error) {
	ac, err := s.accessManager()
	if err != nil {
		return nil, err
	}
	author, err := s.requireAdmin(ctx, "delete grants")
	if err != nil {
		return nil, err
	}
	if err := ac.DeleteGrant(ctx, input.Uri); err != nil {
		if errors.Is(err, access.ErrNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "NotFound: no grant at that URI")
		}
		log.Error(ctx, "failed to delete access grant", "err", err, "uri", input.Uri)
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	log.Log(ctx, "access grant deleted", "uri", input.Uri, "by", author)
	return &placestream.AccessDeleteGrant_Output{Success: true}, nil
}

func (s *Server) handlePlaceStreamAccessUpdatePolicy(ctx context.Context, input *placestream.AccessUpdatePolicy_Input) (*placestream.AccessUpdatePolicy_Output, error) {
	ac, err := s.accessManager()
	if err != nil {
		return nil, err
	}
	author, err := s.requireAdmin(ctx, "update the access policy")
	if err != nil {
		return nil, err
	}
	if err := ac.UpdatePolicy(ctx, input.Roles); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidRole: "+err.Error())
	}
	log.Log(ctx, "access policy updated", "roles", input.Roles, "by", author)
	return &placestream.AccessUpdatePolicy_Output{Policy: policyView(ac)}, nil
}
