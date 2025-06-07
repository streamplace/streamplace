package rtcrec

import (
	"io"
	"os"
	"testing"
	"time"

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
		Offer: &Offer{
			SDPOffer: "test-offer",
		},
		Time: time.Now().UTC(),
	}
	recorder.Event(offerEvent)

	// Test recording an answer event
	answerEvent := WebRTCEvent{
		CreateAnswer: &CreateAnswer{
			SDPAnswer: "test-answer",
		},
		Time: time.Now().UTC(),
	}
	recorder.Event(answerEvent)

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
		var ev WebRTCEvent
		err = dec.Decode(&ev)
		if err == nil {
			evs = append(evs, ev)
		}
	}

	require.ErrorIs(t, err, io.EOF)

	off, ok := evs[0].Detail().(*Offer)
	require.True(t, ok)
	ans, ok := evs[1].Detail().(*CreateAnswer)
	require.True(t, ok)

	require.Equal(t, 2, len(evs))
	require.Equal(t, off.SDPOffer, offerEvent.Offer.SDPOffer)
	require.Equal(t, ans.SDPAnswer, answerEvent.CreateAnswer.SDPAnswer)

}
