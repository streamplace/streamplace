package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"stream.place/streamplace/pkg/muxl"
)

// ValidateReplicatedRenditions verifies both inputs and compares ingredient labels.
// Labels do not prove cryptographic equality of the embedded source manifest.
// This uncommitted cutover must not ship until MUXL exposes authenticated source
// identity or verifies the cross-input binding internally. Neither input is
// published by this function.
func (mm *MediaManager) ValidateReplicatedRenditions(ctx context.Context, streamer string, source, payload []byte) error {
	if _, err := mm.ValidateArchivedMP4(ctx, bytes.NewReader(source), streamer); err != nil {
		return fmt.Errorf("rendition source: %w", err)
	}
	verified := func(input []byte) ([]segmentValidation, error) {
		out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(input))
		if err != nil {
			return nil, err
		}
		var doc struct {
			Segments []segmentValidation `json:"segments"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			return nil, err
		}
		if len(doc.Segments) == 0 {
			return nil, fmt.Errorf("no signed rendition segments")
		}
		for _, segment := range doc.Segments {
			if segment.ValidationState == "Invalid" {
				return nil, fmt.Errorf("track %d has invalid signature", segment.TrackID)
			}
			if active := segment.ValidationResults.ActiveManifest; active != nil && len(active.Failure) != 0 {
				return nil, fmt.Errorf("track %d manifest validation failed", segment.TrackID)
			}
			for _, delta := range segment.ValidationResults.IngredientDeltas {
				if len(delta.ValidationDeltas.Failure) != 0 {
					return nil, fmt.Errorf("track %d ingredient validation failed", segment.TrackID)
				}
			}
		}
		return doc.Segments, nil
	}
	sources, err := verified(source)
	if err != nil {
		return fmt.Errorf("verify rendition source labels: %w", err)
	}
	labels := make(map[string]bool, len(sources))
	for _, segment := range sources {
		if segment.Manifest.Label != nil {
			labels[*segment.Manifest.Label] = true
		}
	}
	renditions, err := verified(payload)
	if err != nil {
		return fmt.Errorf("verify renditions: %w", err)
	}
	for _, segment := range renditions {
		bound := false
		for _, ingredient := range segment.Manifest.Ingredients {
			if ingredient.ActiveManifest != nil && labels[*ingredient.ActiveManifest] {
				bound = true
			}
		}
		if !bound {
			return fmt.Errorf("rendition track %d has no ingredient bound to %s", segment.TrackID, streamer)
		}
	}
	return nil
}
