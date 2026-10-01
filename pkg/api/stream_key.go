package api

import (
	"context"
	"fmt"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func (a *StreamplaceAPI) MakeMediaSigner(ctx context.Context, keyStr string) (media.MediaSigner, error) {
	did, signer, err := media.AuthenticateStreamKey(ctx, a.CLI, a.Model, a.ATSync, keyStr, false)
	if err != nil {
		return nil, err
	}

	mediaSigner, err := media.MakeMediaSigner(ctx, a.CLI, did, signer, a.Model)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization key (not valid secp256k1): %w", err)
	}

	return mediaSigner, nil
}

func (a *StreamplaceAPI) checkBanned(ctx context.Context, did string) error {
	labels, err := a.Model.GetActiveLabels(did)
	if err != nil {
		return fmt.Errorf("failed to get active labels: %w", err)
	}
	if atproto.IsBanned(labels...) {
		log.Error(ctx, "user is banned", "did", did)
		return fmt.Errorf("user is banned")
	}
	return nil
}
