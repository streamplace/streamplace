package rtcrec

import (
	"io"
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

	// err = recorder.Close()
	// require.NoError(t, err)
	err = tmpfile.Close()
	require.NoError(t, err)

	tmpfile, err = os.Open(tmpfile.Name())
	require.NoError(t, err)
	defer tmpfile.Close()

	dec := cbor.NewDecoder(tmpfile)

	evs := []WebRTCEvent{}
	err = nil
	for err == nil {
		ev := WebRTCEvent{}
		err = dec.Decode(&ev)
		if err == nil {
			evs = append(evs, ev)
		}
	}

	require.ErrorIs(t, err, io.EOF)

	require.Equal(t, 2, len(evs))
	require.Equal(t, offerEvent.Offer, evs[0].Offer)
	require.Equal(t, answerEvent.Answer, evs[1].Answer)
}
