package captions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// odd adds odd parity to a 7-bit CEA-608 byte.
func odd(b byte) byte {
	n := 0
	for i := range 7 {
		if b&(1<<i) != 0 {
			n++
		}
	}
	if n%2 == 0 {
		return b | 0x80
	}
	return b
}

func cc1(b1, b2 byte) []byte { return []byte{0xfc, odd(b1), odd(b2)} }

func ctrl1(b1, b2 byte) []byte { return append(cc1(b1, b2), cc1(b1, b2)...) }

func text1(s string) []byte {
	var out []byte
	b := []byte(s)
	for i := 0; i < len(b); i += 2 {
		b2 := byte(0)
		if i+1 < len(b) {
			b2 = b[i+1]
		}
		out = append(out, cc1(b[i], b2)...)
	}
	return out
}

// seiSample wraps cc_data in an SEI NAL inside a length-prefixed access
// unit with an IDR slice.
func seiSample(cc ...[]byte) []byte {
	var data []byte
	for _, c := range cc {
		data = append(data, c...)
	}
	t35 := []byte{0xb5, 0x00, 0x31, 'G', 'A', '9', '4', 0x03, 0x40 | byte(len(data)/3), 0xff}
	t35 = append(t35, data...)
	t35 = append(t35, 0xff)
	nal := []byte{0x06, 0x04, byte(len(t35))}
	nal = append(nal, t35...)
	nal = append(nal, 0x80)
	slice := []byte{0x65, 0x88, 0x84, 0x00}
	return append(prefixed(slice), prefixed(nal)...)
}

func prefixed(nal []byte) []byte {
	return append([]byte{0, 0, 0, byte(len(nal))}, nal...)
}

func TestIngestTapPublishesDisplayStatesAsCues(t *testing.T) {
	hub := NewHub(time.Minute)
	rec := record(hub, "did:plc:s")
	tap := NewIngestTap("did:plc:s", hub, OriginCanonical, "did:plc:s", "en")
	tap.Publish(true)

	require.False(t, tap.Sample(prefixed([]byte{0x65, 0x88}), t0), "no SEI, no captions")
	require.False(t, tap.Seen())

	// Pop-on: load, then flip at t0+1s.
	require.True(t, tap.Sample(seiSample(ctrl1(0x14, 0x20), ctrl1(0x14, 0x70), text1("HELLO")), t0))
	require.False(t, tap.Seen(), "loaded text is not on screen yet")
	tap.Sample(seiSample(ctrl1(0x14, 0x2f)), t0.Add(time.Second))
	require.True(t, tap.Seen())
	// Replace at t0+3s, clear at t0+4.5s.
	tap.Sample(seiSample(ctrl1(0x14, 0x20), ctrl1(0x14, 0x70), text1("AGAIN"), ctrl1(0x14, 0x2f)), t0.Add(3*time.Second))
	tap.Sample(seiSample(ctrl1(0x14, 0x2c)), t0.Add(4500*time.Millisecond))
	rec.wait()

	finals := rec.finals()
	require.Len(t, finals, 2)
	require.Equal(t, "HELLO", finals[0].Text)
	require.Equal(t, t0.Add(time.Second), finals[0].Start)
	require.Equal(t, t0.Add(3*time.Second), finals[0].End, "a cue ends when the display changes")
	require.Equal(t, "AGAIN", finals[1].Text)
	require.Equal(t, t0.Add(4500*time.Millisecond), finals[1].End)
	interims := rec.interims()
	require.Len(t, interims, 2, "each cue is announced when it appears")
	require.Equal(t, finals[0].ID, interims[0].ID)
	require.Equal(t, t0.Add(time.Second+ingestCueHold), interims[0].End, "an interim cue has a provisional end")
	require.Equal(t, Track{ID: "canonical-ingest-en", Language: "en", Kind: KindCaptions, Source: SourceIngest, Origin: OriginCanonical, Label: "Captions", Author: "did:plc:s"}, rec.events[0].Track)
}

func TestIngestTapFollowsOneChannelAndHonoursPublish(t *testing.T) {
	hub := NewHub(time.Minute)
	rec := record(hub, "did:plc:s")
	tap := NewIngestTap("did:plc:s", hub, OriginSidecar, "did:web:node", "")
	// Not publishing yet: decoding still tracks what is on screen.
	tap.Sample(seiSample(ctrl1(0x14, 0x25), ctrl1(0x14, 0x70), text1("ROLL")), t0)
	require.True(t, tap.Seen())
	tap.Publish(true)
	// CC2 text on another channel is ignored once CC1 was chosen.
	tap.Sample(seiSample(ctrl1(0x1c, 0x25), ctrl1(0x1c, 0x70), text1("OTHER")), t0.Add(time.Second))
	tap.Sample(seiSample(ctrl1(0x14, 0x2d), text1("UP")), t0.Add(2*time.Second))
	tap.Close(t0.Add(5 * time.Second))
	rec.wait()

	finals := rec.finals()
	require.Len(t, finals, 2)
	require.Equal(t, "ROLL", finals[0].Text)
	require.Equal(t, t0, finals[0].Start, "the cue that appeared before publishing still ends with its real time")
	require.Equal(t, t0.Add(2*time.Second), finals[0].End)
	require.Equal(t, "ROLL\nUP", finals[1].Text)
	require.Equal(t, t0.Add(2*time.Second), finals[1].Start)
	require.Equal(t, t0.Add(5*time.Second), finals[1].End, "close ends the open cue")
	require.Equal(t, "sidecar-ingest-und", rec.events[0].Track.ID)
	for _, e := range rec.events {
		require.NotContains(t, e.Cue.Text, "OTHER")
	}
}
