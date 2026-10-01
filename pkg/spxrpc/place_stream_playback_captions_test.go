package spxrpc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/vod"
)

const (
	capStreamer = "did:plc:streamer"
	capVideoURI = "at://did:plc:owner/place.stream.video/vid1"
)

var capT0 = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

var enTrack = captions.Track{
	ID: "canonical-auto-en", Language: "en", Kind: captions.KindCaptions,
	Source: captions.SourceAuto, Origin: captions.OriginCanonical, Label: "English",
}

var deTrack = captions.Track{
	ID: "sidecar-auto-de", Language: "de", Kind: captions.KindCaptions,
	Source: captions.SourceAuto, Origin: captions.OriginSidecar,
}

// fakeVideoCaptions is a captions.VideoCaptions over fixed data.
type fakeVideoCaptions struct {
	tracks map[string][]captions.Track
	cues   map[string][]captions.TimedCue // video + "|" + track
	err    error
}

func (f fakeVideoCaptions) Tracks(_ context.Context, video string) ([]captions.Track, error) {
	return f.tracks[video], f.err
}

func (f fakeVideoCaptions) Cues(_ context.Context, video, trackID string) ([]captions.TimedCue, error) {
	return f.cues[video+"|"+trackID], f.err
}

func capServer(t *testing.T) *Server {
	t.Helper()
	m := newTestModel(t)
	require.NoError(t, m.UpsertVideo(context.Background(), placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         "vid",
		Source: placestream.Video_Source{
			MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        []comatproto.RepoStrongRef{{Uri: "at://did:plc:owner/place.stream.media.track/1", Cid: "bafyrefcid"}},
			},
		},
	}, mustATURI(t, capVideoURI)))
	return &Server{
		cli:   &config.CLI{CaptionsMasterDelay: 1500 * time.Millisecond},
		model: m,
		bus:   bus.NewBus(),
	}
}

func mustATURI(t *testing.T, s string) syntax.ATURI {
	t.Helper()
	u, err := syntax.ParseATURI(s)
	require.NoError(t, err)
	return u
}

func getCaptionsReq(s *Server, query string) (*httptest.ResponseRecorder, error) {
	req := httptest.NewRequest(http.MethodGet, "/xrpc/place.stream.caption.getCaptions?"+query, nil)
	rec := httptest.NewRecorder()
	return rec, s.HandleGetCaptions(echo.New().NewContext(req, rec))
}

func httpCode(t *testing.T, err error) int {
	t.Helper()
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	return he.Code
}

func setCaptionQueryWindowPublished(t *testing.T, s *Server, did string, published bool) time.Time {
	t.Helper()
	if s.mm == nil {
		ldb, err := localdb.MakeDB(":memory:")
		require.NoError(t, err)
		s.mm, err = media.MakeMediaManager(context.Background(), s.cli, nil, s.model, s.bus, nil, ldb)
		require.NoError(t, err)
	}
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	frag, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "test", "fixtures", "h264-opus-frag.mp4"))
	require.NoError(t, err)
	rendition, err := muxl.RunMuxlCanonicalize(context.Background(), frag, nil)
	require.NoError(t, err)
	s.mm.FeedLiveRenditions(context.Background(), did, rendition, time.Now(), published)
	w := s.mm.GetLiveWindow(did)
	require.NotNil(t, w)
	require.Equal(t, published, s.mm.LiveWindowPublished(did))
	return w.SubtitleEpoch()
}

// --- live subtitles in the master and media playlists -----------------------

// liveWindow is a window of n one-second segments starting at capT0.
func liveWindow(t *testing.T, n int) *livehls.Writer {
	t.Helper()
	w := livehls.NewWriter()
	require.NoError(t, w.Observe(&muxl.MuxlEvent{
		Type:       "init",
		TrackInits: map[string][]byte{"1": []byte("VINIT")},
		Catalog: &muxl.MuxlCatalog{Video: &muxl.MuxlCatalogVideo{Renditions: map[string]muxl.MuxlVideoConfig{
			"v": {Codec: "avc1.640028", Container: muxl.MuxlContainer{Kind: "cmaf", Timescale: 90000, TrackID: 1}, CodedWidth: 1280, CodedHeight: 720},
		}}},
	}))
	for i := range n {
		require.NoError(t, w.ObserveAt(&muxl.MuxlEvent{
			Type:             "segment",
			Tracks:           map[string][]byte{"1": []byte("v")},
			Durations:        map[string]uint64{"1": 90000},
			SampleCounts:     map[string]uint32{"1": 30},
			FirstDecodeTimes: map[string]uint64{"1": uint64(i) * 90000},
		}, capT0.Add(time.Duration(i)*time.Second)))
	}
	return w
}

func publishFinal(s *Server, tr captions.Track, id string, from, to time.Duration, text string) {
	publishFinalAt(s, capT0, tr, id, from, to, text)
}

func publishFinalAt(s *Server, epoch time.Time, tr captions.Track, id string, from, to time.Duration, text string) {
	s.bus.Captions.Publish(capStreamer, tr, captions.Cue{ID: id, Start: epoch.Add(from), End: epoch.Add(to), Text: text, Final: true})
}

func TestLiveSubtitleRenditionsFollowTheTrackWatermark(t *testing.T) {
	s := capServer(t)
	w := liveWindow(t, 4)

	// No caption tracks: the master has no subtitles, and the playlist
	// of an unknown track is empty.
	require.Empty(t, s.liveSubtitleRenditions(w, capStreamer, "sid", false))
	require.Empty(t, s.liveSubtitlePlaylist(w, capStreamer, enTrack.ID, "sid"))

	// A track whose cues have not reached the end of any segment, in a
	// window younger than the latency budget, is not offered yet.
	publishFinal(s, enTrack, "c0", 100*time.Millisecond, 600*time.Millisecond, "early")
	require.Empty(t, s.liveSubtitleRenditions(w, capStreamer, "sid", false))
	require.Empty(t, s.liveSubtitlePlaylist(w, capStreamer, enTrack.ID, "sid"))

	// A cue running past the end of segments 0 and 1 (ending at 2.5s)
	// shows the track has caught up with them: both are listed.
	publishFinal(s, enTrack, "c1", 900*time.Millisecond, 2500*time.Millisecond, "across")
	publishFinal(s, deTrack, "d1", 100*time.Millisecond, 300*time.Millisecond, "nur kurz")

	subs := s.liveSubtitleRenditions(w, capStreamer, "S1D", false)
	require.Len(t, subs, 1, "the German track has not caught up with any segment")
	require.Equal(t, "English", subs[0].Name)
	require.Equal(t, "en", subs[0].Language)
	u, err := url.Parse(subs[0].URI)
	require.NoError(t, err)
	require.Equal(t, "/xrpc/place.stream.playback.getLivePlaylist", u.Path)
	require.Equal(t, url.Values{"streamer": {capStreamer}, "captions": {enTrack.ID}, "sid": {"S1D"}}, u.Query())

	master := w.MasterPlaylistWithSubtitles(func(tid string) string { return "t" + tid }, subs)
	require.Contains(t, master, `#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="cc",NAME="English",LANGUAGE="en",AUTOSELECT=YES,DEFAULT=NO,URI="/xrpc/place.stream.playback.getLivePlaylist?`)
	require.Contains(t, master, `SUBTITLES="cc"`)

	pl := s.liveSubtitlePlaylist(w, capStreamer, enTrack.ID, "S1D")
	require.Contains(t, pl, "#EXT-X-MEDIA-SEQUENCE:0\n")
	require.Equal(t, 2, strings.Count(pl, "#EXTINF:1.000000,"), pl)
	require.NotContains(t, pl, "ENDLIST")
	var segLines []string
	for _, l := range strings.Split(pl, "\n") {
		if strings.HasPrefix(l, "/xrpc/") {
			segLines = append(segLines, l)
		}
	}
	require.Equal(t, []string{
		liveCaptionSegmentURL(capStreamer, enTrack.ID, 0, "S1D"),
		liveCaptionSegmentURL(capStreamer, enTrack.ID, 1, "S1D"),
	}, segLines)
}

func TestLiveCaptionSegmentURL(t *testing.T) {
	got := liveCaptionSegmentURL(capStreamer, "canonical-auto-en", 17, "SID")
	require.True(t, strings.HasSuffix(got, "&seg=17.vtt"), "ends in .vtt for ffmpeg's extension allowlist: %s", got)
	u, err := url.Parse(got)
	require.NoError(t, err)
	require.Equal(t, "/xrpc/place.stream.playback.getLiveSegment", u.Path)
	q := u.Query()
	require.Equal(t, capStreamer, q.Get("streamer"))
	require.Equal(t, "canonical-auto-en", q.Get("captions"))
	require.Equal(t, "SID", q.Get("sid"))
	require.Equal(t, "17.vtt", q.Get("seg"))
}

// --- listTracks / getCaptions -----------------------------------------------

func TestListTracksLive(t *testing.T) {
	s := capServer(t)
	setCaptionQueryWindowPublished(t, s, capStreamer, true)
	publishFinal(s, deTrack, "d", 0, time.Second, "hallo")
	publishFinal(s, enTrack, "e", 0, time.Second, "hello")

	out, err := s.handlePlaceStreamCaptionListTracks(context.Background(), capStreamer, "")
	require.NoError(t, err)
	require.Len(t, out.Tracks, 2)
	require.Equal(t, "canonical-auto-en", out.Tracks[0].Id)
	require.Equal(t, "English", *out.Tracks[0].Label)
	require.Equal(t, "canonical", out.Tracks[0].Origin)
	require.Equal(t, "auto", out.Tracks[0].Source)
	require.Equal(t, "sidecar-auto-de", out.Tracks[1].Id)
	require.Nil(t, out.Tracks[1].Label)

	_, err = s.handlePlaceStreamCaptionListTracks(context.Background(), "did:plc:nobody", "")
	require.Equal(t, http.StatusNotFound, httpCode(t, err))
}

func TestListTracksAndGetCaptionsParams(t *testing.T) {
	s := capServer(t)
	ctx := context.Background()

	_, err := s.handlePlaceStreamCaptionListTracks(ctx, "", "")
	require.Equal(t, http.StatusBadRequest, httpCode(t, err))
	_, err = s.handlePlaceStreamCaptionListTracks(ctx, capStreamer, capVideoURI)
	require.Equal(t, http.StatusBadRequest, httpCode(t, err))
	_, err = s.handlePlaceStreamCaptionListTracks(ctx, "", "at://did:plc:owner/place.stream.video/missing")
	require.Equal(t, http.StatusNotFound, httpCode(t, err))
	_, err = s.handlePlaceStreamCaptionListTracks(ctx, "", "at://did:plc:owner/app.bsky.feed.post/x")
	require.Equal(t, http.StatusBadRequest, httpCode(t, err))

	// A video with no caption provider has no tracks.
	out, err := s.handlePlaceStreamCaptionListTracks(ctx, "", capVideoURI)
	require.NoError(t, err)
	require.Empty(t, out.Tracks)

	for name, q := range map[string]string{
		"no track":     "streamer=" + capStreamer,
		"both sources": "track=t&streamer=" + capStreamer + "&video=" + capVideoURI,
		"no source":    "track=t",
		"bad format":   "track=t&streamer=" + capStreamer + "&format=ass",
		"bad start":    "track=t&video=" + capVideoURI + "&start=soon",
		"bad mpegts":   "track=t&video=" + capVideoURI + "&mpegts=-1",
	} {
		_, err := getCaptionsReq(s, q)
		require.Equal(t, http.StatusBadRequest, httpCode(t, err), name)
	}
	_, err = getCaptionsReq(s, "track=nope&streamer="+capStreamer)
	require.Equal(t, http.StatusNotFound, httpCode(t, err))
}

func TestLiveCaptionQueriesHideUnpublishedCues(t *testing.T) {
	s := capServer(t)
	s.cli.WideOpen = true // query APIs have no preview-session exception
	setCaptionQueryWindowPublished(t, s, capStreamer, false)
	publishFinal(s, enTrack, "private", 0, time.Second, "not public")

	_, err := s.handlePlaceStreamCaptionListTracks(context.Background(), capStreamer, "")
	var he *echo.HTTPError
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusNotFound, he.Code)
	require.Equal(t, "StreamNotLive", he.Message)

	_, err = getCaptionsReq(s, "streamer="+capStreamer+"&track="+enTrack.ID)
	require.ErrorAs(t, err, &he)
	require.Equal(t, http.StatusNotFound, he.Code)
	require.Equal(t, "StreamNotLive", he.Message)
}

func TestGetCaptionsLive(t *testing.T) {
	s := capServer(t)
	epoch := setCaptionQueryWindowPublished(t, s, capStreamer, true)
	publishFinalAt(s, epoch, enTrack, "c1", 1000*time.Millisecond, 2500*time.Millisecond, "Hello <there> & welcome")
	publishFinalAt(s, epoch, enTrack, "c2", 3000*time.Millisecond, 4000*time.Millisecond, "second line")
	s.bus.Captions.Publish(capStreamer, enTrack, captions.Cue{ID: "interim", Start: epoch.Add(5 * time.Second), End: epoch.Add(6 * time.Second), Text: "typing"})

	rec, err := getCaptionsReq(s, "streamer="+capStreamer+"&track="+enTrack.ID)
	require.NoError(t, err)
	require.Equal(t, "text/vtt; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, `attachment; filename=did-plc-streamer-canonical-auto-en.vtt`, rec.Header().Get("Content-Disposition"))
	require.Equal(t, "WEBVTT\n\n"+
		"c1\n00:00:01.000 --> 00:00:02.500\nHello &lt;there&gt; &amp; welcome\n\n"+
		"c2\n00:00:03.000 --> 00:00:04.000\nsecond line\n\n", rec.Body.String(),
		"final cues only, offset from the published live window's epoch")

	rec, err = getCaptionsReq(s, "streamer="+capStreamer+"&track="+enTrack.ID+"&format=srt")
	require.NoError(t, err)
	require.Equal(t, "application/x-subrip; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), ".srt")
	require.Equal(t, "1\n00:00:01,000 --> 00:00:02,500\nHello <there> & welcome\n\n2\n00:00:03,000 --> 00:00:04,000\nsecond line\n\n", rec.Body.String())

	rec, err = getCaptionsReq(s, "streamer="+capStreamer+"&track="+enTrack.ID+"&format=json")
	require.NoError(t, err)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), ".json")
	var doc struct {
		Epoch string `json:"epoch"`
		Cues  []struct {
			ID      string `json:"id"`
			StartMs int64  `json:"startMs"`
			EndMs   int64  `json:"endMs"`
			Text    string `json:"text"`
		} `json:"cues"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &doc))
	require.Equal(t, epoch.Format(time.RFC3339Nano), doc.Epoch)
	require.Len(t, doc.Cues, 2)
	require.Equal(t, int64(3000), doc.Cues[1].StartMs)
}

func TestGetCaptionsVideo(t *testing.T) {
	s := capServer(t)
	s.VideoCaptions = fakeVideoCaptions{
		tracks: map[string][]captions.Track{capVideoURI: {{ID: "record-human-en", Language: "en", Source: captions.SourceHuman, Origin: captions.OriginRecord}}},
		cues: map[string][]captions.TimedCue{capVideoURI + "|record-human-en": {
			{ID: "a", Start: 0, End: 1500 * time.Millisecond, Text: "first"},
			{ID: "b", Start: 1500 * time.Millisecond, End: 3500 * time.Millisecond, Text: "spans the cut"},
			{ID: "c", Start: 4 * time.Second, End: 5 * time.Second, Text: "last"},
		}},
	}

	out, err := s.handlePlaceStreamCaptionListTracks(context.Background(), "", capVideoURI)
	require.NoError(t, err)
	require.Len(t, out.Tracks, 1)
	require.Equal(t, "record", out.Tracks[0].Origin)

	// A download: everything, with an attachment filename named for the video.
	rec, err := getCaptionsReq(s, "video="+url.QueryEscape(capVideoURI)+"&track=record-human-en")
	require.NoError(t, err)
	require.Equal(t, "attachment; filename=vid1-record-human-en.vtt", rec.Header().Get("Content-Disposition"))
	require.NotContains(t, rec.Body.String(), "X-TIMESTAMP-MAP")
	for _, text := range []string{"first", "spans the cut", "last"} {
		require.Contains(t, rec.Body.String(), text)
	}

	// An HLS segment: cues overlapping [2s, 4s), on the video timeline, with
	// the timestamp map, no download disposition, and the URL's cosmetic
	// .vtt tolerated on the track.
	rec, err = getCaptionsReq(s, "video="+url.QueryEscape(capVideoURI)+"&format=vtt&start=2000&end=4000&mpegts=900000&track=record-human-en.vtt")
	require.NoError(t, err)
	require.Empty(t, rec.Header().Get("Content-Disposition"))
	require.Equal(t, "WEBVTT\nX-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:00:00:00.000\n\n"+
		"b\n00:00:01.500 --> 00:00:03.500\nspans the cut\n\n", rec.Body.String(),
		"the cue ending exactly at the start and the one starting exactly at the end are outside [start, end)")

	_, err = getCaptionsReq(s, "video="+url.QueryEscape(capVideoURI)+"&track=other")
	require.Equal(t, http.StatusNotFound, httpCode(t, err))
}

// --- VOD subtitle playlist ----------------------------------------------------

func capBox(typ string, payload ...[]byte) []byte {
	var body []byte
	for _, p := range payload {
		body = append(body, p...)
	}
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], typ)
	return append(b, body...)
}

// capSegment is a signed canonical segment as it sits in a VOD blob: signature
// boxes, then the moof with the segment's tfdt, then the mdat.
func capSegment(tfdt uint32) []byte {
	p := make([]byte, 8)
	binary.BigEndian.PutUint32(p[4:], tfdt)
	return append(append(capBox("uuid", make([]byte, 3000)), capBox("moof", capBox("traf", capBox("tfdt", p)))...), capBox("mdat", make([]byte, 64))...)
}

// vodCaptionFixture is a video of three 2s segments (6000 ticks/s) in a flat
// blob behind a 500 byte header. The first session starts at media time 10s;
// segment 2 starts a second session, whose tfdt restarted at 0.5s.
func vodCaptionFixture(t *testing.T) (*Server, *vod.Metafile) {
	t.Helper()
	s := capServer(t)
	segs := [][]byte{capSegment(60000), capSegment(72000), capSegment(3000)}
	blobBytes := make([]byte, 500)
	meta := &vod.Metafile{
		BlobCID:        "bafyblob",
		FlatHeaderSize: 500,
		Tracks: map[string]vod.MetafileTrack{"1": {
			Type: "video", Codec: "avc1.64002a", Timescale: 6000, BlobCID: "bafyblob", Width: 1280, Height: 720,
		}},
	}
	tr := meta.Tracks["1"]
	var off int64
	for i, seg := range segs {
		tr.Segments = append(tr.Segments, vod.MetafileSegment{Offset: off, Size: int64(len(seg)), DurationTicks: 12000, SampleCount: 60, Discontinuity: i == 2})
		off += int64(len(seg))
		blobBytes = append(blobBytes, seg...)
	}
	meta.Tracks["1"] = tr

	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	w, err := store.NewWriter(context.Background(), vod.BlobsPrefix+"bafyblob.mp4", "video/mp4")
	require.NoError(t, err)
	_, err = w.Write(blobBytes)
	require.NoError(t, err)
	require.NoError(t, w.Complete())
	s.playbackStore = store
	s.VideoCaptions = fakeVideoCaptions{tracks: map[string][]captions.Track{capVideoURI: {
		{ID: "record-human-en", Language: "en", Source: captions.SourceHuman, Origin: captions.OriginRecord, Label: "English"},
		{ID: "record-imported-de", Language: "de", Source: captions.SourceImported, Origin: captions.OriginRecord},
	}}}
	return s, meta
}

func TestVODMasterPlaylistSubtitlesGroup(t *testing.T) {
	s, meta := vodCaptionFixture(t)
	start := int64(1000)
	subs := s.vodSubtitleRenditions(context.Background(), capVideoURI, "SID", &start, nil)
	require.Len(t, subs, 2)
	require.Equal(t, "English", subs[0].Name)
	require.Equal(t, "de (imported)", subs[1].Name, "an unlabeled track is named by language and source")

	pl := masterPlaylist(meta, capVideoURI, "SID", &start, nil, subs)
	require.Equal(t, 2, strings.Count(pl, "#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"cc\""))
	require.Equal(t, 1, strings.Count(pl, "#EXT-X-STREAM-INF"))
	require.Contains(t, pl, `SUBTITLES="cc"`)
	u, err := url.Parse(subs[0].URI)
	require.NoError(t, err)
	require.Equal(t, "/xrpc/place.stream.playback.getVideoPlaylist", u.Path)
	require.Equal(t, url.Values{"uri": {capVideoURI}, "captions": {"record-human-en"}, "sid": {"SID"}, "start": {"1000"}}, u.Query(),
		"the clip bounds travel with the subtitle playlist like the media playlist's")

	require.NotContains(t, masterPlaylist(meta, capVideoURI, "SID", nil, nil, nil), "SUBTITLES")
}

func TestVODSubtitlePlaylistAlignsToVideoSegments(t *testing.T) {
	s, meta := vodCaptionFixture(t)
	pl, err := s.vodSubtitlePlaylist(context.Background(), meta, capVideoURI, "record-human-en", 0, nil, nil)
	require.NoError(t, err)

	require.Contains(t, pl, "#EXT-X-PLAYLIST-TYPE:VOD\n")
	require.Contains(t, pl, "#EXT-X-MEDIA-SEQUENCE:0\n")
	require.Contains(t, pl, "#EXT-X-ENDLIST\n")
	require.Equal(t, 1, strings.Count(pl, "#EXT-X-DISCONTINUITY\n"), "the video playlist's boundary is repeated")

	var uris []*url.URL
	for _, l := range strings.Split(pl, "\n") {
		if strings.HasPrefix(l, "/xrpc/") {
			require.True(t, strings.HasSuffix(l, "&track=record-human-en.vtt"), "ends in .vtt: %s", l)
			u, err := url.Parse(l)
			require.NoError(t, err)
			require.Equal(t, "/xrpc/place.stream.caption.getCaptions", u.Path)
			uris = append(uris, u)
		}
	}
	require.Len(t, uris, 3)
	for i, want := range []struct{ start, end, mpegts string }{
		// Session one: media time 10s at video 0s.
		{"0", "2000", "900000"},
		{"2000", "4000", "900000"},
		// Session two: tfdt 0.5s at video 4s: -3.5s, wrapped to 33 bits.
		{"4000", "6000", "8589619592"},
	} {
		q := uris[i].Query()
		require.Equal(t, capVideoURI, q.Get("video"))
		require.Equal(t, "vtt", q.Get("format"))
		require.Equal(t, want.start, q.Get("start"), i)
		require.Equal(t, want.end, q.Get("end"), i)
		require.Equal(t, want.mpegts, q.Get("mpegts"), i)
	}

	// The same segments the video's own playlist lists, in the same order.
	video, err := mediaPlaylist(meta, "1", "did:plc:owner", "SID", vodCDN{}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, strings.Count(video, "#EXTINF:2.000000,"), strings.Count(pl, "#EXTINF:2.000000,"))
	require.Equal(t, strings.Count(video, "#EXT-X-DISCONTINUITY\n"), strings.Count(pl, "#EXT-X-DISCONTINUITY\n"))
}

func TestVODSubtitlePlaylistOfClip(t *testing.T) {
	s, meta := vodCaptionFixture(t)
	// A clip of the parent from 5s: segment 2 (the second session) is
	// first; the clip's timeline begins at 5s of the parent's.
	start := int64(5000)
	pl, err := s.vodSubtitlePlaylist(context.Background(), meta, capVideoURI, "record-human-en", 5000, &start, nil)
	require.NoError(t, err)
	require.Contains(t, pl, "#EXT-X-DISCONTINUITY-SEQUENCE:1\n")
	require.Equal(t, 1, strings.Count(pl, "#EXTINF"))
	var line string
	for _, l := range strings.Split(pl, "\n") {
		if strings.HasPrefix(l, "/xrpc/") {
			line = l
		}
	}
	u, err := url.Parse(line)
	require.NoError(t, err)
	q := u.Query()
	// Segment 2 began 1s before the clip: clip offsets -1s to 1s.
	require.Equal(t, "0", q.Get("start"))
	require.Equal(t, "1000", q.Get("end"))
	// tfdt 0.5s at clip offset -1s: MPEGTS = 1.5s.
	require.Equal(t, "135000", q.Get("mpegts"))
}

func TestVODSubtitlePlaylistErrors(t *testing.T) {
	s, meta := vodCaptionFixture(t)
	ctx := context.Background()

	_, err := s.vodSubtitlePlaylist(ctx, meta, capVideoURI, "no-such-track", 0, nil, nil)
	require.Equal(t, http.StatusNotFound, httpCode(t, err))

	s.VideoCaptions = nil
	_, err = s.vodSubtitlePlaylist(ctx, meta, capVideoURI, "record-human-en", 0, nil, nil)
	require.Equal(t, http.StatusNotFound, httpCode(t, err))
	require.Empty(t, s.vodSubtitleRenditions(ctx, capVideoURI, "SID", nil, nil), "no provider: no subtitles, playback unaffected")

	// A provider failure costs the captions, not the video.
	s.VideoCaptions = fakeVideoCaptions{err: context.DeadlineExceeded}
	require.Empty(t, s.vodSubtitleRenditions(ctx, capVideoURI, "SID", nil, nil))
}

// A blob that cannot be read for timing still yields a playlist: the
// timeline is taken to start at zero.
func TestVODSubtitlePlaylistWithoutBlobTiming(t *testing.T) {
	s, meta := vodCaptionFixture(t)
	s.playbackStore = nil
	pl, err := s.vodSubtitlePlaylist(context.Background(), meta, capVideoURI, "record-human-en", 0, nil, nil)
	require.NoError(t, err)
	var mpegts []string
	for _, l := range strings.Split(pl, "\n") {
		if strings.HasPrefix(l, "/xrpc/") {
			u, err := url.Parse(l)
			require.NoError(t, err)
			mpegts = append(mpegts, u.Query().Get("mpegts"))
		}
	}
	require.Equal(t, []string{"0", "0", "0"}, mpegts)
}
