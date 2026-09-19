package media

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/muxl"
)

// A stand-in for the transcoder: the fixture re-encoded at a smaller size,
// fragmented, one keyframe per second — the shape Livepeer hands back.
func renditionMP4(t *testing.T, w, h int) []byte {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	out := filepath.Join(t.TempDir(), fmt.Sprintf("r%dx%d.mp4", w, h))
	// Two seconds at one keyframe a second: a rendition the transcoder
	// keyframed mid-segment, so minting has to sign per GoP.
	cmd := exec.Command("ffmpeg", "-v", "error", "-y", "-i", getFixture("h264-opus-frag.mp4"),
		"-t", "2", "-an", "-vf", fmt.Sprintf("scale=%d:%d", w, h), "-c:v", "libx264", "-preset", "ultrafast",
		"-g", "30", "-keyint_min", "30", "-sc_threshold", "0", "-pix_fmt", "yuv420p",
		"-movflags", "+frag_keyframe+empty_moov", "-f", "mp4", out)
	require.NoError(t, cmd.Run(), "ffmpeg")
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	return b
}

func nodeSignerForTest(t *testing.T) (cert, keyPEM []byte) {
	t.Helper()
	atPriv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	secpPriv, _ := secp256k1.PrivKeyFromBytes(atPriv.Bytes())
	var signer crypto.Signer = secpPriv.ToECDSA()
	_ = ecdsa.PublicKey{}
	cert, err = signers.GenerateES256KCert(signer)
	require.NoError(t, err)
	keyPEM, err = signers.MarshalES256KPrivateKeyPEM(signer)
	require.NoError(t, err)
	return cert, keyPEM
}

// The fixture signed as a streamer's source segment (the first GoP).
func sourceSegmentForTest(t *testing.T) []byte {
	t.Helper()
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
	var first []byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" && first == nil {
			first = concatTracksByID(ev.Tracks)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, first)
	return first
}

// Renditions of a source segment become signed video tracks at fixed ids,
// verify, and fold into the live window as HLS variants beside the source.
func TestMintVideoRenditions(t *testing.T) {
	ctx := context.Background()
	src := sourceSegmentForTest(t)
	r360 := renditionMP4(t, 160, 120)
	r160 := renditionMP4(t, 80, 60)
	cert, keyPEM := nodeSignerForTest(t)
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "node.test"}, liveWindows: map[string]*livehls.Writer{}}

	addendum, err := mm.mintVideoRenditions(ctx, src, []RenditionInput{{Name: "160p", MP4: r360}, {Name: "80p", MP4: r160}}, cert, keyPEM)
	require.NoError(t, err)
	require.NotEmpty(t, addendum)

	// Signed: every rendition byte verifies, alone and appended to the
	// source segment the way a completed segment is (what a peer validates).
	_, err = muxl.RunMuxlVerify(ctx, bytes.NewReader(addendum))
	require.NoError(t, err, "addendum verifies")
	completed := append(append([]byte{}, src...), addendum...)
	valid, err := ValidateMP4Media(ctx, completed)
	require.NoError(t, err, "source + renditions validates as one segment")
	require.NotNil(t, valid.Manifest.Label)

	// Two video tracks at the fixed rendition ids, sized as transcoded.
	events, err := unwrapMuxlEvents(ctx, addendum)
	require.NoError(t, err)
	sizes := map[uint32][2]uint32{}
	for _, ev := range events {
		if ev.Type == "init" && ev.Catalog != nil && ev.Catalog.Video != nil {
			for _, v := range ev.Catalog.Video.Renditions {
				sizes[v.TrackID()] = [2]uint32{v.CodedWidth, v.CodedHeight}
			}
		}
	}
	require.Equal(t, [2]uint32{160, 120}, sizes[RenditionTrackID(0)])
	require.Equal(t, [2]uint32{80, 60}, sizes[RenditionTrackID(1)])

	// Into the live window: the source and both renditions are variants.
	const did = "did:test:streamer"
	mm.feedLiveWindow(ctx, did, src, true)
	mm.FeedLiveRenditions(ctx, did, addendum, true)
	w := mm.GetLiveWindow(did)
	require.NotNil(t, w)
	master := w.MasterPlaylist(func(tid string) string { return tid + ".m3u8" })
	require.Equal(t, 3, strings.Count(master, "#EXT-X-STREAM-INF"), master)
	require.Contains(t, master, "RESOLUTION=320x240")
	require.Contains(t, master, "RESOLUTION=160x120")
	require.Contains(t, master, "RESOLUTION=80x60")
	for _, tid := range []string{"100", "101"} {
		tr := w.Track(tid)
		require.NotNil(t, tr, "track %s in window", tid)
		require.Len(t, tr.Segments, 2, "two GoPs, two window segments on track %s", tid)
		require.NotEmpty(t, w.InitSegment(tid))
		require.NotEmpty(t, w.SegmentData(tid, tr.Segments[0].Seq))
		pl := w.MediaPlaylist(tid, "init.mp4", func(seq uint64) string { return fmt.Sprintf("s%d.m4s", seq) })
		require.Contains(t, pl, "#EXTINF")
	}

	// A second segment from the same transcoder canonicalizes once (the
	// layout is remembered) and lands on the same track ids.
	again, err := mm.mintVideoRenditions(ctx, src, []RenditionInput{{Name: "160p", MP4: r360}, {Name: "80p", MP4: r160}}, cert, keyPEM)
	require.NoError(t, err)
	mm.FeedLiveRenditions(ctx, did, again, true)
	require.Len(t, w.Track("100").Segments, 4)

	// A broken rendition is skipped, the rest still mint.
	partial, err := mm.mintVideoRenditions(ctx, src, []RenditionInput{{Name: "160p", MP4: r360}, {Name: "bad", MP4: []byte("not an mp4")}}, cert, keyPEM)
	require.NoError(t, err)
	require.NotEmpty(t, partial)
}

func TestRenditionFraming(t *testing.T) {
	addendum := []byte{0, 0, 0, 24, 'u', 'u', 'i', 'd', 1, 2, 3}
	framed := FrameRenditions(addendum)
	got, ok := UnframeRenditions(framed)
	require.True(t, ok)
	require.Equal(t, addendum, got)
	_, ok = UnframeRenditions(addendum)
	require.False(t, ok, "a raw segment (box size first) is never mistaken for a frame")
	_, ok = UnframeRenditions([]byte("SPRN"))
	require.False(t, ok, "an empty frame is not an addendum")
}

// A rendition's fragments are moved onto the source segment's timeline: the
// transcoder saw running time starting at zero, the window needs the
// stream's decode time so the variant lines up with the source.
func TestMintVideoRenditionsRetimes(t *testing.T) {
	ctx := context.Background()
	// The fixture's SECOND signed segment: its decode time is not zero.
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
	var segs [][]byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" {
			segs = append(segs, concatTracksByID(ev.Tracks))
		}
	}
	require.NoError(t, <-errCh)
	require.Greater(t, len(segs), 1)
	src := segs[1]
	srcEvents, err := unwrapMuxlEvents(ctx, src)
	require.NoError(t, err)
	cat, tracks := catalogAndTracks(srcEvents)
	var srcTID, srcTS uint32
	for _, v := range cat.Video.Renditions {
		srcTID, srcTS = v.TrackID(), v.Timescale()
	}
	srcBase, ok := firstTfdt(tracks[fmt.Sprint(srcTID)])
	require.True(t, ok)
	require.NotZero(t, srcBase, "second segment starts after zero")

	cert, keyPEM := nodeSignerForTest(t)
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "node.test"}, liveWindows: map[string]*livehls.Writer{}}
	addendum, err := mm.mintVideoRenditions(ctx, src, []RenditionInput{{Name: "160p", MP4: renditionMP4(t, 160, 120)}}, cert, keyPEM)
	require.NoError(t, err)
	require.NotEmpty(t, addendum)
	renEvents, err := unwrapMuxlEvents(ctx, addendum)
	require.NoError(t, err)
	rcat, rtracks := catalogAndTracks(renEvents)
	var renTS uint32
	for _, v := range rcat.Video.Renditions {
		renTS = v.Timescale()
	}
	renBase, ok := firstTfdt(rtracks[fmt.Sprint(RenditionTrackID(0))])
	require.True(t, ok)
	want := uint64(float64(srcBase) * float64(renTS) / float64(srcTS))
	require.Equal(t, want, renBase, "rendition starts where the source segment starts")
	_, err = muxl.RunMuxlVerify(ctx, bytes.NewReader(addendum))
	require.NoError(t, err, "re-timed before signing, so it still verifies")
}
