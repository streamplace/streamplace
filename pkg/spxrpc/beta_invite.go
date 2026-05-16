package spxrpc

import (
	"context"
	"fmt"
)

// vodInviteFeature is the canonical `feature` value on a
// place.stream.beta.invite record that grants an account VOD upload
// access. New beta features should pick their own short identifier.
const vodInviteFeature = "vod"

// allowVODUpload is the gate that runs on every VOD upload attempt.
// Returns nil when the caller is allowed; otherwise a forbidden-style
// error suitable for surfacing back to the client.
//
// The policy mirrors what the user-facing live-stream gate does:
//
//   - If --beta-invite-did is configured, that account is the sole
//     trusted issuer of upload invites. We require an indexed
//     place.stream.beta.invite record under its repo naming this DID
//     with feature == "vod"; nothing else gets through.
//
//   - If --beta-invite-did is empty (self-hosted / dev), we fall back
//     to cli.StreamIsAllowed — same allowlist livestreaming uses,
//     including the "no allowedStreams configured ⇒ open server"
//     behavior. So a fresh dev node keeps working out of the box and
//     a self-hoster who already locked down SP_ALLOWED_STREAMS for
//     live keeps the same lockdown for uploads.
func (s *Server) allowVODUpload(ctx context.Context, did string) error {
	if s.cli.BetaInviteDID != "" {
		has, err := s.model.HasBetaInvite(ctx, s.cli.BetaInviteDID, did, vodInviteFeature)
		if err != nil {
			return fmt.Errorf("look up beta invite: %w", err)
		}
		if !has {
			return fmt.Errorf("VOD upload is beta-gated; no invite found for %s", did)
		}
		return nil
	}
	if err := s.cli.StreamIsAllowed(did); err != nil {
		return fmt.Errorf("VOD upload not allowed: %w", err)
	}
	return nil
}
