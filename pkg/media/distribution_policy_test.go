package media

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqtime"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/localdb"
)

func manifestWithDistributionPolicy(policy map[string]any) *c2patypes.Manifest {
	return &c2patypes.Manifest{
		Assertions: []c2patypes.ManifestAssertion{
			{
				Label: "place.stream.metadata.configuration",
				Data: map[string]any{
					"distributionPolicy": policy,
				},
			},
		},
	}
}

// extractDistributionPolicy carries allowGenAiTraining out of the manifest, alone
// or alongside deleteAfter, and stays nil when neither is declared.
func TestExtractDistributionPolicyAllowGenAiTraining(t *testing.T) {
	now := aqtime.FromTime(time.Now())

	policy := extractDistributionPolicy(manifestWithDistributionPolicy(map[string]any{
		"allowGenAiTraining": false,
	}), now)
	require.NotNil(t, policy)
	require.NotNil(t, policy.AllowGenAiTraining)
	require.False(t, *policy.AllowGenAiTraining)
	require.Nil(t, policy.DeleteAfterSeconds)

	policy = extractDistributionPolicy(manifestWithDistributionPolicy(map[string]any{
		"allowGenAiTraining": true,
		"deleteAfter":        300,
	}), now)
	require.NotNil(t, policy)
	require.NotNil(t, policy.AllowGenAiTraining)
	require.True(t, *policy.AllowGenAiTraining)
	require.NotNil(t, policy.DeleteAfterSeconds)
	require.Equal(t, int64(300), *policy.DeleteAfterSeconds)

	policy = extractDistributionPolicy(manifestWithDistributionPolicy(map[string]any{
		"allowedBroadcasters": []string{"*"},
	}), now)
	require.Nil(t, policy, "a policy with neither deleteAfter nor allowGenAiTraining stays nil")
}

// The minted place.stream.segment record carries allowGenAiTraining even when
// deleteAfter is unset.
func TestSegmentRecordCarriesAllowGenAiTraining(t *testing.T) {
	notAllowed := false
	seg := &localdb.Segment{
		ID:            "test-segment",
		RepoDID:       "did:example:streamer",
		SigningKeyDID: "did:key:test",
		MediaData: &localdb.SegmentMediaData{
			Video: []*localdb.SegmentMediadataVideo{{Width: 1920, Height: 1080, FPSNum: 30, FPSDen: 1}},
			Audio: []*localdb.SegmentMediadataAudio{{Rate: 48000, Channels: 2}},
		},
		DistributionPolicy: &localdb.DistributionPolicy{
			AllowGenAiTraining: &notAllowed,
		},
	}
	record, err := seg.ToStreamplaceSegment()
	require.NoError(t, err)
	require.NotNil(t, record.DistributionPolicy)
	require.NotNil(t, record.DistributionPolicy.AllowGenAiTraining)
	require.False(t, *record.DistributionPolicy.AllowGenAiTraining)
	require.Nil(t, record.DistributionPolicy.DeleteAfter)
}
