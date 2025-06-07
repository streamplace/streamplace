package rtcrec

import (
	"os"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func TestWebRTCRecording(t *testing.T) {
	// Create a temporary file for testing
	tmpfile, err := os.CreateTemp("", "webrtc-recording-test-*")
	require.NoError(t, err)

	// Create recorder stream writing to temp file
	recorder, err := NewRecorderStream(tmpfile)
	require.NoError(t, err)

	// Test recording an offer event
	offerEvent := WebRTCEvent{
		Offer: &OfferEvent{
			Offer: "test-offer",
		},
	}
	require.NoError(t, recorder.Event(offerEvent))

	// Test recording an answer event
	answerEvent := WebRTCEvent{
		Answer: &AnswerEvent{
			Answer: "test-answer",
		},
	}
	require.NoError(t, recorder.Event(answerEvent))

	err = recorder.Close()
	require.NoError(t, err)
	err = tmpfile.Close()
	require.NoError(t, err)

	// Read the file and verify the contents
	contents, err := os.ReadFile(tmpfile.Name())
	require.NoError(t, err)

	var evs []WebRTCEvent
	err = cbor.Unmarshal(contents, &evs)
	require.NoError(t, err)

	require.Equal(t, 2, len(evs))
	require.Equal(t, offerEvent.Offer, evs[0].Offer)
	require.Equal(t, answerEvent.Answer, evs[1].Answer)
}
