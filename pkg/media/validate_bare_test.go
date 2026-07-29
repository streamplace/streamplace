package media

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/muxl"
)

// newBareSegmentSigner builds a MediaSignerLocal with a fresh ES256K key +
// self-signed cert and a cawg.metadata-bearing prebuilt manifest — enough to
// drive SignSegmentStream in tests.
func newBareSegmentSigner(t *testing.T) *MediaSignerLocal {
	t.Helper()
	atPriv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	secpPriv, _ := secp256k1.PrivKeyFromBytes(atPriv.Bytes())
	require.NotNil(t, secpPriv)
	var signer crypto.Signer = secpPriv.ToECDSA()
	cert, err := signers.GenerateES256KCert(signer)
	require.NoError(t, err)
	pub, err := aqpub.FromPublicKey(secpPriv.ToECDSA().Public().(*ecdsa.PublicKey))
	require.NoError(t, err)
	return &MediaSignerLocal{
		StreamerName: "test-streamer",
		Signer:       signer,
		AQPub:        pub,
		Cert:         cert,
		PrebuiltManifest: []byte(`{
			"title": "bare segment test",
			"assertions": [
				{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}},
				{"label":"cawg.metadata","data":{
					"@context":{"dc":"http://purl.org/dc/elements/1.1/"},
					"dc:creator":"did:example","dc:title":"t",
					"dc:date":"1970-01-01T00:00:00.000Z"
				}}
			]
		}`),
	}
}

// TestValidateMP4MediaBareSegment exercises the full .m4s-native validate
// path: sign the fragmented fixture per-segment (the live ingest shape),
// reassemble one GoP's bare canonical .m4s, and run ValidateMP4Media over it.
// That wraps the bare segment to a flat MP4 for gstreamer (codec/dimensions)
// and verifies the signatures in-wasm — proving a signed bare .m4s parses
// through qtdemux with the correct co64 offsets wrap-flat synthesizes.
func TestValidateMP4MediaBareSegment(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)

	frag, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)

	// Sign per-segment; keep the first GoP's bare .m4s (what ValidateMP4 gets
	// per call in the live path).
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(frag), eventCh)
		close(eventCh)
		errCh <- err
	}()
	var m4s []byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" && m4s == nil {
			m4s = concatTracksSorted(ev.Tracks)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, m4s, "expected at least one signed GoP")

	res, err := ValidateMP4Media(ctx, m4s)
	require.NoError(t, err)
	require.NotNil(t, res.MediaData)
	require.NotEmpty(t, res.MediaData.Video, "should parse a video track")
	require.Greater(t, res.MediaData.Video[0].Width, 0, "video width from qtdemux")
	require.Greater(t, res.MediaData.Video[0].Height, 0, "video height from qtdemux")
	require.NotEmpty(t, res.MediaData.Audio, "should parse an audio track")
}

// TestFeedLiveWindow signs the fixture per-segment, folds the resulting bare
// .m4s into a MediaManager live-HLS window via feedLiveWindow, and confirms the
// window is populated per track with retrievable signed segments + valid
// playlists — the feed path ValidateMP4 drives for every validated segment.
func TestFeedLiveWindow(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	frag, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)

	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(frag), eventCh)
		close(eventCh)
		errCh <- err
	}()
	var m4s []byte
	for ev := range eventCh {
		if ev.Type != "signed-segment" {
			continue
		}
		tids := make([]string, 0, len(ev.Tracks))
		for tid := range ev.Tracks {
			tids = append(tids, tid)
		}
		sort.Strings(tids)
		for _, tid := range tids {
			m4s = append(m4s, ev.Tracks[tid]...)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, m4s)

	mm := &MediaManager{liveWindows: map[string]*livehls.Writer{}}

	// Pre-live (unpublished) segments must NOT be folded into the live window —
	// live HLS is unauthenticated, so anything in the window is world-readable.
	mm.feedLiveWindow(ctx, "did:test:streamer", m4s, false)
	require.Nil(t, mm.GetLiveWindow("did:test:streamer"),
		"unpublished segment must not create a live-HLS window")

	mm.feedLiveWindow(ctx, "did:test:streamer", m4s, true)

	w := mm.GetLiveWindow("did:test:streamer")
	require.NotNil(t, w, "window created on feed")
	tids := w.TrackIDs()
	require.NotEmpty(t, tids, "window has tracks")
	for _, tid := range tids {
		require.NotEmpty(t, w.InitSegment(tid), "track %s has an init segment", tid)
		tr := w.Track(tid)
		require.NotEmpty(t, tr.Segments, "track %s has segments", tid)
		data := w.SegmentData(tid, tr.Segments[0].Seq)
		require.GreaterOrEqual(t, len(data), 8)
		require.Equal(t, "uuid", string(data[4:8]), "track %s segment is the signed .m4s", tid)
		pl := w.MediaPlaylist(tid, "init.mp4", func(seq uint64) string { return fmt.Sprintf("seg%d.m4s", seq) })
		require.Contains(t, pl, "#EXT-X-MAP")
		require.Contains(t, pl, "#EXTINF")
	}
	require.Contains(t, w.MasterPlaylist(func(tid string) string { return tid + ".m3u8" }), "#EXTM3U")
}

// seedBanLabel writes an active ban label for did into the indexdb.
func seedBanLabel(t *testing.T, mod indexdb.Model, did string) {
	t.Helper()
	lex := &comatproto.LabelDefs_Label{
		Cts: time.Now().UTC().Format(time.RFC3339),
		Src: "did:plc:test-labeler",
		Uri: did,
		Val: atproto.LabelDMCAViolation,
	}
	var buf bytes.Buffer
	require.NoError(t, lex.MarshalCBOR(&buf))
	require.NoError(t, mod.CreateLabel(&indexdb.Label{
		Src:    lex.Src,
		Uri:    did,
		Val:    atproto.LabelDMCAViolation,
		Record: buf.Bytes(),
	}))
}

// TestStreamerIsBanned exercises the defense-in-depth gate validateSource applies
// at the ingest chokepoint: a streamer with an active ban label is rejected
// regardless of whether their ingest worker was torn down. A clean streamer
// passes; the ONLY change between the two checks is the ban label.
func TestStreamerIsBanned(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	did := "did:plc:bannedstreamer"

	banned, err := mm.streamerIsBanned(did)
	require.NoError(t, err)
	require.False(t, banned, "a clean streamer is not banned")

	seedBanLabel(t, mm.model, did)

	banned, err = mm.streamerIsBanned(did)
	require.NoError(t, err)
	require.True(t, banned, "an active ban label is detected at the validate chokepoint")
}
