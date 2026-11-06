package misttriggers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var streamBufferPayloadFull = MistTriggerBody(`
	stream1
	FULL
	{"track1":{"codec":"h264","kbits":1000,"keys":{"B":"1"},"fpks":30,"height":720,"width":1280},"jitter":420}`)

var streamBufferPayloadIssues = MistTriggerBody(`
	stream1
	RECOVER
	{"track1":{"codec":"h264","kbits":1000,"keys":{"B":"1"},"fpks":30,"height":720,"width":1280},"issues":"The aqueous linear entity, in a manner pertaining to its metaphorical state of existence, appears to be experiencing an ostensibly suboptimal condition that is reminiscent of an individual's disposition when subjected to an unfavorable meteorological phenomenon","human_issues":["Stream is feeling under the weather"]}
`)

var streamBufferPayloadInvalid = MistTriggerBody(`
	stream1
	FULL
	{"track1":{},"notatrack":{"codec":2}}
`)

var streamBufferPayloadEmpty = MistTriggerBody(`
	stream1
	EMPTY
`)

func TestItCanParseAValidStreamBufferPayload(t *testing.T) {
	p, err := ParseStreamBufferPayload(streamBufferPayloadFull)
	require.NoError(t, err)
	require.Equal(t, p.StreamName, "stream1")
	require.Equal(t, p.State, "FULL")
	require.NotNil(t, p.Details)
	require.Equal(t, p.Details.Issues, "")
	require.Len(t, p.Details.Tracks, 1)
	require.Contains(t, p.Details.Tracks, "track1")
	require.Equal(t, p.Details.Extra["jitter"], float64(420))
}

func TestItCanParseAStreamBufferPayloadWithStreamIssues(t *testing.T) {
	p, err := ParseStreamBufferPayload(streamBufferPayloadIssues)
	require.NoError(t, err)
	require.Equal(t, p.StreamName, "stream1")
	require.Equal(t, p.State, "RECOVER")
	require.NotNil(t, p.Details)
	require.Equal(t, p.Details.HumanIssues, []string{"Stream is feeling under the weather"})
	require.Contains(t, p.Details.Issues, "unfavorable meteorological phenomenon")
	require.Len(t, p.Details.Tracks, 1)
	require.Contains(t, p.Details.Tracks, "track1")
}

func TestItCanParseAValidStreamBufferPayloadWithEmptyState(t *testing.T) {
	p, err := ParseStreamBufferPayload(streamBufferPayloadEmpty)
	require.NoError(t, err)
	require.Equal(t, p.StreamName, "stream1")
	require.Equal(t, p.State, "EMPTY")
	require.Nil(t, p.Details)
}

func TestItFailsToParseAnInvalidStreamBufferPayload(t *testing.T) {
	_, err := ParseStreamBufferPayload(streamBufferPayloadInvalid)
	require.Error(t, err)
}
