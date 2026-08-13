package media

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// manifestModelStub satisfies model.Model for ManifestBuilder tests; only the
// two methods BuildManifest calls are implemented.
type manifestModelStub struct {
	model.Model
	metadata *model.MetadataConfiguration
}

func (m *manifestModelStub) GetLatestLivestreamForRepo(repoDID string) (*model.Livestream, error) {
	return nil, nil
}

func (m *manifestModelStub) GetMetadataConfiguration(ctx context.Context, repoDID string) (*model.MetadataConfiguration, error) {
	return m.metadata, nil
}

func metadataConfigurationRow(t *testing.T, cfg *placestream.MetadataConfiguration) *model.MetadataConfiguration {
	var buf bytes.Buffer
	require.NoError(t, cfg.MarshalCBOR(&buf))
	bs := buf.Bytes()
	return &model.MetadataConfiguration{
		RepoDID: "did:example:streamer",
		Record:  &bs,
	}
}

func buildManifestAssertions(t *testing.T, metadata *model.MetadataConfiguration) map[string]map[string]any {
	mb := NewManifestBuilder(&manifestModelStub{metadata: metadata}, &config.CLI{})
	manifestBs, err := mb.BuildManifest(context.Background(), "did:example:streamer", 1700000000000)
	require.NoError(t, err)

	var mani struct {
		Assertions []struct {
			Label string         `json:"label"`
			Data  map[string]any `json:"data"`
		} `json:"assertions"`
	}
	require.NoError(t, json.Unmarshal(manifestBs, &mani))

	out := map[string]map[string]any{}
	for _, a := range mani.Assertions {
		out[a.Label] = a.Data
	}
	return out
}

func aiTrainingUse(t *testing.T, assertions map[string]map[string]any) string {
	data, ok := assertions["cawg.training-mining"]
	require.True(t, ok, "manifest must carry the cawg.training-mining assertion")
	entries, ok := data["entries"].(map[string]any)
	require.True(t, ok)
	entry, ok := entries["cawg.ai_generative_training"].(map[string]any)
	require.True(t, ok)
	use, ok := entry["use"].(string)
	require.True(t, ok)
	return use
}

func allowAiTrainingField(t *testing.T, assertions map[string]map[string]any) any {
	data, ok := assertions["place.stream.metadata.configuration"]
	require.True(t, ok, "manifest must carry the metadata configuration assertion")
	policy, ok := data["distributionPolicy"].(map[string]any)
	require.True(t, ok, "metadata configuration must carry a distributionPolicy")
	return policy["allowAiTraining"]
}

// A streamer with no metadata configuration at all still gets an explicit
// "AI training not allowed" stamped into every minted segment.
func TestBuildManifestAiTrainingDefaultDeny(t *testing.T) {
	assertions := buildManifestAssertions(t, nil)
	require.Equal(t, "notAllowed", aiTrainingUse(t, assertions))
	require.Equal(t, false, allowAiTrainingField(t, assertions))
}

// A metadata configuration that never mentions AI training also defaults to
// "not allowed" — the preference is opt-in.
func TestBuildManifestAiTrainingUndeclaredDeny(t *testing.T) {
	deleteAfter := int64(300)
	row := metadataConfigurationRow(t, &placestream.MetadataConfiguration{
		DistributionPolicy: &placestream.MetadataDistributionPolicy{
			DeleteAfter: &deleteAfter,
		},
	})
	assertions := buildManifestAssertions(t, row)
	require.Equal(t, "notAllowed", aiTrainingUse(t, assertions))
	require.Equal(t, false, allowAiTrainingField(t, assertions))

	// the rest of the distribution policy survives the stamping
	policy := assertions["place.stream.metadata.configuration"]["distributionPolicy"].(map[string]any)
	require.Equal(t, float64(300), policy["deleteAfter"])
}

// An explicit opt-in is honored.
func TestBuildManifestAiTrainingExplicitAllow(t *testing.T) {
	allow := true
	row := metadataConfigurationRow(t, &placestream.MetadataConfiguration{
		DistributionPolicy: &placestream.MetadataDistributionPolicy{
			AllowAiTraining: &allow,
		},
	})
	assertions := buildManifestAssertions(t, row)
	require.Equal(t, "allowed", aiTrainingUse(t, assertions))
	require.Equal(t, true, allowAiTrainingField(t, assertions))
}
