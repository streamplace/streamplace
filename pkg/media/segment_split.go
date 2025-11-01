package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"stream.place/streamplace/pkg/aqio"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/log"
)

type SplitSegment struct {
	Filename string
	Data     []byte
}

type ManifestResult struct {
	Manifests map[string]c2patypes.Manifest `json:"manifests"`
	Certs     map[string]string             `json:"certs"`
}

type ManifestAndMetadata struct {
	Manifest        c2patypes.Manifest
	SegmentMetadata *SegmentMetadata
}

// split a signed concatenated mp4 into its constituent signed segments
func SplitSegments(ctx context.Context, input []byte) ([]SplitSegment, error) {
	manifestsStr, err := iroh_streamplace.GetManifests(c2patypes.NewReader(aqio.NewReadWriteSeeker(input)))
	if err != nil {
		return nil, fmt.Errorf("failed to get manifests: %w", err)
	}
	var manifests ManifestResult
	err = json.Unmarshal([]byte(manifestsStr), &manifests)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifests: %w", err)
	}
	manifestList := []ManifestAndMetadata{}
	for _, manifest := range manifests.Manifests {
		metadata, err := ParseSegmentAssertions(context.Background(), &manifest)
		if err == ErrMissingMetadata {
			log.Error(ctx, "missing metadata", "manifest", manifest.Label)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse segment assertions: %w", err)
		}
		manifestList = append(manifestList, ManifestAndMetadata{
			Manifest:        manifest,
			SegmentMetadata: metadata,
		})
	}
	sort.Slice(manifestList, func(i, j int) bool {
		m1 := manifestList[i]
		m2 := manifestList[j]
		return m1.SegmentMetadata.StartTime.Time().Before(m2.SegmentMetadata.StartTime.Time())
	})
	manifestStrs := []string{}
	certList := [][]byte{}
	for _, manifest := range manifestList {
		manifestStrs = append(manifestStrs, *manifest.Manifest.Label)
		certList = append(certList, []byte(manifests.Certs[*manifest.Manifest.Label]))
	}
	unsignedSegs, err := SegmentUnsigned(ctx, bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("failed to segment file: %w", err)
	}
	resignedSegs, err := iroh_streamplace.Resign(unsignedSegs, c2patypes.NewReader(aqio.NewReadWriteSeeker(input)), manifestStrs, certList)
	if err != nil {
		return nil, fmt.Errorf("failed to resign segments: %w", err)
	}
	splitSegments := []SplitSegment{}
	for i, resignedSeg := range resignedSegs {
		meta := manifestList[i].SegmentMetadata
		fname := fmt.Sprintf("%s.mp4", meta.StartTime.FileSafeString())
		splitSegments = append(splitSegments, SplitSegment{
			Filename: fname,
			Data:     resignedSeg,
		})
	}
	return splitSegments, nil
}
