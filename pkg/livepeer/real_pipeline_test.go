package livepeer

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/renditions"
)

// The gateway push converts with GStreamer; initialise it once for the package.
func TestMain(m *testing.M) {
	gstinit.InitGST()
	os.Exit(m.Run())
}

// TestRealBroadcastPipeline runs a stored live recording (a concatenation
// of bare canonical MUXL segments, e.g. one live-rec S3 object) through the
// real transcode path segment by segment — the Opus presentation, the
// MP4→TS conversion, the gateway push, the TS→MP4 remux, rendition minting
// (relabel + sign), verification, and the live window — and reports what
// happened to every segment. Offline and unpaced: a gateway must be
// running (the ffmpeg mock or a real one).
//
//	SP_REAL_M4S=/path/to/recording.m4s [SP_REAL_MAX=200] [SP_REAL_GATEWAY=http://127.0.0.1:38666] [SP_REAL_HEIGHT=160] [SP_REAL_OUT=dir]
func TestRealBroadcastPipeline(t *testing.T) {
	path := os.Getenv("SP_REAL_M4S")
	if path == "" {
		t.Skip("SP_REAL_M4S not set")
	}
	max := 1 << 30
	if v := os.Getenv("SP_REAL_MAX"); v != "" {
		max, _ = strconv.Atoi(v)
	}
	gw := os.Getenv("SP_REAL_GATEWAY")
	if gw == "" {
		gw = "http://127.0.0.1:38666"
	}
	height := int64(160)
	if v := os.Getenv("SP_REAL_HEIGHT"); v != "" {
		h, _ := strconv.Atoi(v)
		height = int64(h)
	}
	outDir := os.Getenv("SP_REAL_OUT")
	// SP_REAL_SCAN=1: read, regroup and inspect only — no gateway, no mint.
	// An inventory of a recording: durations, tiny segments, duplicated GoPs.
	scanOnly := os.Getenv("SP_REAL_SCAN") != ""

	ctx := context.Background()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	st, err := f.Stat()
	require.NoError(t, err)
	size := st.Size()

	cli := &config.CLI{BroadcasterHost: "harness.test"}
	mm := media.NewOffline(cli)
	atPriv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	secpPriv, _ := secp256k1.PrivKeyFromBytes(atPriv.Bytes())
	var signer crypto.Signer = secpPriv.ToECDSA()
	cert, err := signers.GenerateES256KCert(signer)
	require.NoError(t, err)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(signer)
	require.NoError(t, err)
	ls, err := NewLivepeerSession(ctx, cli, "did:test:real", gw)
	require.NoError(t, err)
	w := livehls.NewWriter()

	type row struct {
		seq          int
		srcTfdt      uint64
		dup          bool
		srcBytes     int
		dur          float64
		samples      uint32
		audio        string
		tiny         bool
		convMS, gwMS int64
		mintMS       int64
		renBytes     int
		renDur       float64
		renSamples   uint32
		verified     bool
		err          string
	}
	var rows []row
	var offset int64
	feed := func(seg []byte) error {
		ch := make(chan *muxl.MuxlEvent, 16)
		errCh := make(chan error, 1)
		go func() { errCh <- muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(seg), ch); close(ch) }()
		for ev := range ch {
			if err := w.Observe(ev); err != nil {
				return err
			}
		}
		return <-errCh
	}
	unwrap := func(seg []byte) ([]*muxl.MuxlEvent, error) {
		ch := make(chan *muxl.MuxlEvent, 16)
		errCh := make(chan error, 1)
		go func() { errCh <- muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(seg), ch); close(ch) }()
		var out []*muxl.MuxlEvent
		for ev := range ch {
			out = append(out, ev)
		}
		return out, <-errCh
	}
	// A live recording stores each GoP as one signed asset per track, in
	// track order (video, then the audio codecs); ReadSegments hands back one
	// asset at a time. Regroup them into whole segments — video plus its
	// audio — which is what the node's validate path receives.
	hasVideo := func(asset []byte) bool {
		evs, err := unwrap(asset)
		if err != nil {
			return false
		}
		for _, ev := range evs {
			if ev.Type == "init" && ev.Catalog != nil && ev.Catalog.Video != nil && len(ev.Catalog.Video.Renditions) > 0 {
				return true
			}
		}
		return false
	}
	var pending []byte
	nextSegment := func() ([]byte, bool) {
		for offset < size {
			asset, err := muxl.RunMuxlReadSegmentsAtFileOffset(ctx, f, size, offset, 1)
			if err != nil || len(asset) == 0 {
				t.Logf("read at offset %d failed: %v — stopping", offset, err)
				offset = size
				break
			}
			offset += int64(len(asset))
			if hasVideo(asset) && len(pending) > 0 {
				seg := pending
				pending = append([]byte{}, asset...)
				return seg, true
			}
			pending = append(pending, asset...)
		}
		if len(pending) > 0 {
			seg := pending
			pending = nil
			return seg, true
		}
		return nil, false
	}
	started := time.Now()
	for i := 0; i < max; i++ {
		seg, more := nextSegment()
		if !more {
			break
		}
		r := row{seq: i, srcBytes: len(seg)}
		evs, err := unwrap(seg)
		if err != nil {
			r.err = "unwrap: " + err.Error()
			rows = append(rows, r)
			continue
		}
		var vw, vh, vts uint32
		var audios []string
		for _, ev := range evs {
			if ev.Type == "init" && ev.Catalog != nil {
				if ev.Catalog.Video != nil {
					for _, v := range ev.Catalog.Video.Renditions {
						vw, vh, vts = v.CodedWidth, v.CodedHeight, v.Timescale()
					}
				}
				if ev.Catalog.Audio != nil {
					for _, a := range ev.Catalog.Audio.Renditions {
						audios = append(audios, a.Codec)
					}
				}
			}
			if (ev.Type == "segment" || ev.Type == "signed-segment") && r.samples == 0 {
				for tid, d := range ev.Durations {
					if tid == "1" || r.samples == 0 {
						if vts > 0 {
							r.dur = float64(d) / float64(vts)
						}
						r.samples = ev.SampleCounts[tid]
					}
				}
			}
		}
		r.audio = strings.Join(audios, "+")
		r.tiny = r.dur < 0.5
		// The source video track's first decode time: a segment that starts
		// where the previous one did is the same interval encoded again (a
		// stream pushed twice into one recording).
		for _, ev := range evs {
			if ev.Type == "segment" || ev.Type == "signed-segment" {
				if vb := ev.Tracks["1"]; len(vb) > 0 {
					r.srcTfdt, _ = firstTfdtOf(vb)
				}
				break
			}
		}
		if len(rows) > 0 && rows[len(rows)-1].srcTfdt == r.srcTfdt && r.srcTfdt != 0 {
			r.dup = true
		}
		if scanOnly {
			rows = append(rows, r)
			continue
		}
		if vw == 0 || vh == 0 {
			r.err = fmt.Sprintf("no video in catalog (tracks: %d events, audio=%q)", len(evs), r.audio)
			rows = append(rows, r)
			continue
		}
		if err := feed(seg); err != nil {
			r.err = "window(src): " + err.Error()
		}
		durNs := int64(r.dur * float64(time.Second))
		fps := int64(25)
		if r.dur > 0 {
			fps = int64(float64(r.samples)/r.dur + 0.5)
		}
		spseg := &placestream.Segment{
			Creator:   "did:test:real",
			StartTime: time.Now().UTC().Format(time.RFC3339Nano),
			Duration:  &durNs,
			Video:     []placestream.Segment_Video{{Width: int64(vw), Height: int64(vh), Framerate: &placestream.Segment_Framerate{Num: fps, Den: 1}}},
		}
		rw := int64(vw) * height / int64(vh)
		rs := renditions.Renditions{{Name: fmt.Sprintf("%dp", height), Width: rw, Height: height, Bitrate: 250_000, Framerate: renditions.FPS{Num: uint(fps), Den: 1}, Profile: "h264baseline"}}

		t0 := time.Now()
		flat, err := media.PresentationWithOpus(ctx, seg)
		if err != nil {
			r.err = "presentation: " + err.Error()
			rows = append(rows, r)
			continue
		}
		r.convMS = time.Since(t0).Milliseconds()
		if dump := os.Getenv("SP_REAL_DUMP"); dump != "" && i < 2 {
			// Every stage of one segment, for frame-counting by hand.
			_ = os.MkdirAll(dump, 0o755)
			_ = os.WriteFile(filepath.Join(dump, fmt.Sprintf("seg%d-src.m4s", i)), seg, 0o644)
			_ = os.WriteFile(filepath.Join(dump, fmt.Sprintf("seg%d-flat.mp4", i)), flat, 0o644)
			var ts, audio bytes.Buffer
			if err := media.MP4ToMPEGTSVideoMP4Audio(ctx, bytes.NewReader(flat), &ts, &audio); err == nil {
				_ = os.WriteFile(filepath.Join(dump, fmt.Sprintf("seg%d-sent.ts", i)), ts.Bytes(), 0o644)
			}
		}
		t0 = time.Now()
		outs, err := ls.PostSegmentToGateway(ctx, flat, spseg, rs)
		r.gwMS = time.Since(t0).Milliseconds()
		if dump := os.Getenv("SP_REAL_DUMP"); dump != "" && i < 2 && err == nil && len(outs) > 0 {
			_ = os.WriteFile(filepath.Join(dump, fmt.Sprintf("seg%d-returned.mp4", i)), outs[0], 0o644)
		}
		if err != nil {
			r.err = "gateway: " + err.Error()
			rows = append(rows, r)
			continue
		}
		t0 = time.Now()
		addendum, err := mm.MintVideoRenditionsWith(ctx, seg, []media.RenditionInput{{Name: rs[0].Name, MP4: outs[0]}}, cert, keyPEM)
		r.mintMS = time.Since(t0).Milliseconds()
		if err != nil || addendum == nil {
			r.err = fmt.Sprintf("mint: %v (addendum %d bytes)", err, len(addendum))
			rows = append(rows, r)
			continue
		}
		if dump := os.Getenv("SP_REAL_DUMP"); dump != "" && i < 2 {
			_ = os.WriteFile(filepath.Join(dump, fmt.Sprintf("seg%d-addendum.m4s", i)), addendum, 0o644)
		}
		r.renBytes = len(addendum)
		if _, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(append(append([]byte{}, seg...), addendum...))); err != nil {
			r.err = "verify: " + err.Error()
		} else {
			r.verified = true
		}
		if aevs, err := unwrap(addendum); err == nil {
			for _, ev := range aevs {
				if ev.Type == "segment" || ev.Type == "signed-segment" {
					for tid, d := range ev.Durations {
						if vts > 0 {
							r.renDur += float64(d) / float64(vts)
						}
						r.renSamples += ev.SampleCounts[tid]
						_ = tid
					}
				}
			}
		}
		if err := feed(addendum); err != nil {
			r.err = "window(ren): " + err.Error()
		}
		if r.renDur < r.dur-0.05 || r.renSamples != r.samples {
			t.Logf("segment %d: source %.3fs/%d frames, rendition %.3fs/%d frames", r.seq, r.dur, r.samples, r.renDur, r.renSamples)
		}
		rows = append(rows, r)
		if (i+1)%10 == 0 {
			t.Logf("…%d segments in %s", i+1, time.Since(started).Round(time.Second))
		}
	}

	// Report.
	var ok, tiny, failed, dups int
	var srcDur, renDur float64
	var convSum, gwSum, mintSum int64
	reasons := map[string]int{}
	for _, r := range rows {
		srcDur += r.dur
		renDur += r.renDur
		convSum += r.convMS
		gwSum += r.gwMS
		mintSum += r.mintMS
		if r.tiny {
			tiny++
		}
		if r.dup {
			dups++
		}
		if scanOnly {
			continue
		}
		if r.err == "" && r.verified {
			ok++
		} else {
			failed++
			key := r.err
			if len(key) > 90 {
				key = key[:90]
			}
			reasons[key]++
		}
	}
	n := len(rows)
	t.Logf("SOURCE %s: %d segments, %.1fs of media, %d tiny (<0.5s), %d duplicated GoPs (same start as the previous segment), audio=%q", filepath.Base(path), n, srcDur, tiny, dups, rows[0].audio)
	if scanOnly {
		return
	}
	t.Logf("RESULT ok=%d failed=%d rendition media=%.1fs (%.1f%% of source)", ok, failed, renDur, 100*renDur/srcDur)
	if n > 0 {
		t.Logf("TIMING avg per segment: presentation %dms, gateway (conv+transcode+remux) %dms, mint+sign %dms", convSum/int64(n), gwSum/int64(n), mintSum/int64(n))
	}
	for k, v := range reasons {
		t.Logf("FAIL x%d: %s", v, k)
	}
	master := w.MasterPlaylist(func(tid string) string { return tid + ".m3u8" })
	t.Logf("WINDOW variants=%d tracks=%v", strings.Count(master, "#EXT-X-STREAM-INF"), w.TrackIDs())
	for _, tid := range w.TrackIDs() {
		tr := w.Track(tid)
		if tr != nil {
			t.Logf("  track %s %s %s %dx%d segments=%d", tid, tr.Type, tr.Codec, tr.Width, tr.Height, len(tr.Segments))
		}
	}
	if outDir != "" {
		_ = os.MkdirAll(outDir, 0o755)
		for _, tid := range w.TrackIDs() {
			tr := w.Track(tid)
			if tr == nil || len(tr.Segments) == 0 {
				continue
			}
			var buf bytes.Buffer
			buf.Write(w.InitSegment(tid))
			for j, s := range tr.Segments {
				if j >= 20 {
					break
				}
				buf.Write(w.SegmentData(tid, s.Seq))
			}
			p := filepath.Join(outDir, "track-"+tid+".mp4")
			require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o644))
			t.Logf("  wrote %s (%d bytes, first 20 segments)", p, buf.Len())
		}
	}
	require.Greater(t, ok, 0, "at least one segment must make it through")
}

// firstTfdtOf is the first fragment's baseMediaDecodeTime in a run of
// [uuid]…[moof][mdat] boxes.
func firstTfdtOf(b []byte) (uint64, bool) {
	var val uint64
	found := false
	walkBoxRange(b, 0, len(b), func(typ string, s, e int) bool {
		if typ != "moof" {
			return true
		}
		walkBoxRange(b, s+8, e, func(t2 string, s2, e2 int) bool {
			if t2 != "traf" {
				return true
			}
			walkBoxRange(b, s2+8, e2, func(t3 string, s3, e3 int) bool {
				if t3 != "tfdt" || e3-s3 < 16 {
					return true
				}
				if b[s3+8] == 1 {
					val = uint64(be32(b, s3+12))<<32 | uint64(be32(b, s3+16))
				} else {
					val = uint64(be32(b, s3+12))
				}
				found = true
				return false
			})
			return !found
		})
		return !found
	})
	return val, found
}

func be32(b []byte, off int) uint32 {
	return uint32(b[off])<<24 | uint32(b[off+1])<<16 | uint32(b[off+2])<<8 | uint32(b[off+3])
}

// walkBoxRange calls fn(type, start, end) for each box in b[start:end]
// until fn returns false.
func walkBoxRange(b []byte, start, end int, fn func(typ string, s, e int) bool) {
	for off := start; off+8 <= end; {
		size := int(be32(b, off))
		if size < 8 || off+size > end {
			return
		}
		if !fn(string(b[off+4:off+8]), off, off+size) {
			return
		}
		off += size
	}
}
