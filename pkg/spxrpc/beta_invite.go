package spxrpc

import (
	"context"
	"fmt"
)

// vodInviteFeature is the canonical `feature` value on a
// place.stream.beta.invite record that grants an account VOD upload
// access. New beta features should pick their own short identifier.
const vodInviteFeature = "vod"

// betaFeatureGranted reports whether `did` may use `feature`. This is the
// single source of truth that both the upload gate and the
// place.stream.beta.getStatus query consult, so the gate and the status the
// UI shows can never disagree.
//
// The policy mirrors what the user-facing live-stream gate does:
//
//   - If --beta-invite-did is configured, that account is the sole
//     trusted issuer of feature invites. We require an indexed
//     place.stream.beta.invite record under its repo naming this DID
//     with the given feature; nothing else gets through.
//
//   - If --beta-invite-did is empty (self-hosted / dev), we fall back
//     to cli.StreamIsAllowed — same allowlist livestreaming uses,
//     including the "no allowedStreams configured ⇒ open server"
//     behavior. So a fresh dev node keeps working out of the box and
//     a self-hoster who already locked down SP_ALLOWED_STREAMS for
//     live keeps the same lockdown for uploads.
func (s *Server) betaFeatureGranted(ctx context.Context, did, feature string) (bool, error) {
	if s.cli.BetaInviteDID != "" {
		has, err := s.model.HasBetaInvite(ctx, s.cli.BetaInviteDID, did, feature)
		if err != nil {
			return false, fmt.Errorf("look up beta invite: %w", err)
		}
		return has, nil
	}
	return s.cli.StreamIsAllowed(did) == nil, nil
}

// betaFeatureStatus reports an account's access status for a feature as one of
// "granted", "requested", or "none". "granted" folds together both a trusted
// invite and the open self-hosted fallback (see betaFeatureGranted); a pending
// place.stream.beta.request downgrades "none" to "requested".
func (s *Server) betaFeatureStatus(ctx context.Context, did, feature string) (string, error) {
	granted, err := s.betaFeatureGranted(ctx, did, feature)
	if err != nil {
		return "", err
	}
	if granted {
		return "granted", nil
	}
	requested, err := s.model.HasBetaRequest(ctx, did, feature)
	if err != nil {
		return "", fmt.Errorf("look up beta request: %w", err)
	}
	if requested {
		return "requested", nil
	}
	return "none", nil
}

// allowVODUpload is the gate that runs on every VOD upload attempt.
// Returns nil when the caller is allowed; otherwise a forbidden-style
// error suitable for surfacing back to the client.
func (s *Server) allowVODUpload(ctx context.Context, did string) error {
	granted, err := s.betaFeatureGranted(ctx, did, vodInviteFeature)
	if err != nil {
		return err
	}
	if !granted {
		if s.cli.BetaInviteDID != "" {
			return fmt.Errorf("VOD upload is beta-gated; no invite found for %s", did)
		}
		return fmt.Errorf("VOD upload not allowed for %s", did)
	}
	return nil
}
