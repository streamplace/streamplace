package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/aigateway"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/spmetrics"
)

func (a *StreamplaceAPI) NormalizeUser(ctx context.Context, user string) (string, error) {
	alias, ok := a.Aliases[user]
	if ok {
		user = alias
	}
	// did:key, pass through unaltered
	if strings.HasPrefix(user, constants.DID_KEY_PREFIX) {
		return user, nil
	}
	// only other allowed case is a bluesky handle
	repo, err := a.ATSync.SyncBlueskyRepoCached(ctx, user, a.Model)
	if err != nil {
		return "", err
	}
	return repo.DID, nil
}

func (a *StreamplaceAPI) HandleWebRTCPlayback(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		rendition := getRendition(r)
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "invalid user", err)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "error reading body", err)
			return
		}
		offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: string(body)}
		var answer *webrtc.SessionDescription
		if a.CLI.NewWebRTCPlayback {
			answer, err = a.MediaManager.WebRTCPlayback2(ctx, user, rendition, &offer)
		} else {
			answer, err = a.MediaManager.WebRTCPlayback(ctx, user, rendition, &offer)
		}
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "error playing back", err)
			return
		}
		w.WriteHeader(201)
		w.Header().Add("Location", r.URL.Path)
		if _, err := w.Write([]byte(answer.SDP)); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

const BearerPrefix = "Bearer "
const KeyPrefix = "0x"

func (a *StreamplaceAPI) HandleWebRTCIngest(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/sdp" {
			errors.WriteHTTPBadRequest(w, "invalid content type", nil)
			return
		}
		var encoded string
		urlKey := p.ByName("key")
		if urlKey != "" {
			encoded = urlKey
		} else {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				errors.WriteHTTPUnauthorized(w, "authorization header required", nil)
				return
			}
			if !strings.HasPrefix(auth, BearerPrefix) {
				errors.WriteHTTPUnauthorized(w, "invalid authorization header (needs Bearer prefix)", nil)
				return
			}
			encoded = auth[len(BearerPrefix):]
			// it's easy to copy-paste a trailing or leading space, so clear those out
			encoded = strings.TrimSpace(encoded)
		}

		mediaSigner, err := a.MakeMediaSigner(ctx, encoded)
		if err != nil {
			errors.WriteHTTPUnauthorized(w, "invalid authorization key", err)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "error reading body", err)
			return
		}
		offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: string(body)}
		pc, err := a.MediaManager.NewPeerConnection(ctx, mediaSigner.Streamer())
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to create peer connection", err)
			return
		}
		answer, err := a.MediaManager.WebRTCIngest(ctx, &offer, mediaSigner, pc, make(chan error, 1))
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "error playing back", err)
			return
		}
		host := r.Host
		if host == "" {
			host = r.URL.Host
		}
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		location := fmt.Sprintf("%s://%s/api/live/webrtc", scheme, host)
		log.Log(ctx, "location", "location", location)
		w.Header().Set("Location", location)
		w.WriteHeader(201)
		if _, err := w.Write([]byte(answer.SDP)); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

var epoch = time.Unix(0, 0).Format(time.RFC1123)

var noCacheHeaders = map[string]string{
	"Expires":         epoch,
	"Cache-Control":   "no-cache, private, max-age=0",
	"Pragma":          "no-cache",
	"X-Accel-Expires": "0",
}

var etagHeaders = []string{
	"ETag",
	"If-Modified-Since",
	"If-Match",
	"If-None-Match",
	"If-Range",
	"If-Unmodified-Since",
}

func NoCache(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Delete any ETag headers that may have been set
		for _, v := range etagHeaders {
			if r.Header.Get(v) != "" {
				r.Header.Del(v)
			}
		}

		// Set our NoCache headers
		for k, v := range noCacheHeaders {
			w.Header().Set(k, v)
		}

		h(w, r, p)
	}
}

const SessionExpireTime = 30 * time.Second

func (a *StreamplaceAPI) SessionSeen(ctx context.Context, user string, session string) {
	now := time.Now()
	go func() {
		a.sessionsLock.Lock()
		defer a.sessionsLock.Unlock()
		if _, ok := a.sessions[user]; !ok {
			a.sessions[user] = map[string]time.Time{}
		}
		if _, ok := a.sessions[user][session]; !ok {
			log.Warn(ctx, "ViewerInc", "user", user, "session", session)
			spmetrics.ViewerInc(user, "hls")
			a.Bus.IncrementViewerCount(user, "local")
		}
		a.sessions[user][session] = now
	}()
}

func (a *StreamplaceAPI) ExpireSessions(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			a.sessionsLock.Lock()
			for user, sessions := range a.sessions {
				for session, seen := range sessions {
					if time.Since(seen) > SessionExpireTime {
						delete(sessions, session)
						spmetrics.ViewerDec(user, "hls")
						a.Bus.DecrementViewerCount(user, "local")
					}
				}
			}
			a.sessionsLock.Unlock()
		}
	}
}

func (a *StreamplaceAPI) HandleHLSPlayback(ctx context.Context) httprouter.Handle {
	return NoCache(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "invalid user", err)
			return
		}
		file := p.ByName("file")
		if file == "" {
			errors.WriteHTTPBadRequest(w, "file required", nil)
			return
		}

		file = strings.TrimPrefix(file, "/")

		if strings.HasPrefix(file, "subtitles/") {
			a.handleSubtitles(ctx, w, r, user, file)
			return
		}

		m3u8, err := a.Director.GetM3U8(ctx, user)
		if err != nil {
			errors.WriteHTTPNotFound(w, "could not get m3u8", err)
			return
		}
		session := r.URL.Query().Get("session")
		rendition := r.URL.Query().Get("rendition")
		buf, err := m3u8.GetFile(file, session, rendition)
		if err != nil {
			errors.WriteHTTPNotFound(w, "segment not found", err)
			return
		}

		// Propagate subtitle offset from the master playlist URL into the subtitle track URI.
		// This keeps the subtitle playlist/segments consistently parameterized for hls.js.
		if strings.HasSuffix(file, ".m3u8") && file == media.IndexM3U8 {
			subOffsetMS := strings.TrimSpace(r.URL.Query().Get("sub_offset_ms"))
			if subOffsetMS != "" {
				if _, err := strconv.ParseInt(subOffsetMS, 10, 64); err == nil {
					playlist := string(buf)
					lines := strings.Split(playlist, "\n")
					for i := range lines {
						line := lines[i]
						if !strings.HasPrefix(line, "#EXT-X-MEDIA:TYPE=SUBTITLES") {
							continue
						}
						uriStart := strings.Index(line, "URI=\"")
						if uriStart < 0 {
							continue
						}
						uriStart += len("URI=\"")
						uriEnd := strings.Index(line[uriStart:], "\"")
						if uriEnd < 0 {
							continue
						}
						uriEnd = uriStart + uriEnd
						uri := line[uriStart:uriEnd]
						if strings.Contains(uri, "sub_offset_ms=") {
							continue
						}
						sep := "?"
						if strings.Contains(uri, "?") {
							sep = "&"
						}
						uri = uri + sep + "sub_offset_ms=" + subOffsetMS
						lines[i] = line[:uriStart] + uri + line[uriEnd:]
					}
					buf = []byte(strings.Join(lines, "\n"))
				}
			}
		}

		if strings.HasSuffix(file, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		} else {
			if session != "" {
				a.SessionSeen(ctx, user, session)
			}
			w.Header().Set("Content-Type", "video/mp2t")
		}

		http.ServeContent(w, r, file, time.Now(), bytes.NewReader(buf))
	})
}

func (a *StreamplaceAPI) HandleThumbnailPlayback(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			errors.WriteHTTPNotFound(w, "user not found", err)
			return
		}
		thumb, err := a.Model.LatestThumbnailForUser(user)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "could not query thumbnail", err)
			return
		}
		if thumb == nil {
			errors.WriteHTTPNotFound(w, "thumbnail not found", err)
			return
		}
		aqt := aqtime.FromTime(thumb.Segment.StartTime)
		fpath, err := a.CLI.SegmentFilePath(user, fmt.Sprintf("%s.%s", aqt.String(), thumb.Format))
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "could not get segment file path", err)
			return
		}
		http.ServeFile(w, r, fpath)
	}
}

func (a *StreamplaceAPI) handleSubtitles(ctx context.Context, w http.ResponseWriter, r *http.Request, user string, file string) {
	m3u8, err := a.Director.GetM3U8(ctx, user)
	if err != nil {
		errors.WriteHTTPNotFound(w, "could not get m3u8", err)
		return
	}

	if !m3u8.SubtitlesEnabled() {
		errors.WriteHTTPNotFound(w, "subtitles not enabled", nil)
		return
	}

	subFile := strings.TrimPrefix(file, "subtitles/")
	subOffsetMSStr := strings.TrimSpace(r.URL.Query().Get("sub_offset_ms"))
	subOffsetMS := int64(0)
	if subOffsetMSStr != "" {
		if v, err := strconv.ParseInt(subOffsetMSStr, 10, 64); err == nil {
			subOffsetMS = v
		}
	}

	if subFile == media.IndexM3U8 {
		rend, err := m3u8.GetRendition("source")
		if err != nil {
			errors.WriteHTTPNotFound(w, "could not get source rendition", err)
			return
		}

		rend.SegmentLock.RLock()
		mediaSegs := make([]*media.Segment, len(rend.Segments))
		copy(mediaSegs, rend.Segments)
		rend.SegmentLock.RUnlock()

		// Mirror the source rendition playlist structure so players can align subtitle
		// fragments to the media timeline.
		segCount := len(mediaSegs)
		startWith := segCount - media.LivePlaylistSize
		if startWith < 0 {
			startWith = 0
		}
		if segCount == 0 {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			http.ServeContent(w, r, subFile, time.Now(), bytes.NewReader([]byte("")))
			return
		}

		firstSeg := mediaSegs[startWith]
		lastSeg := mediaSegs[len(mediaSegs)-1]
		targetDur := int64(math.Round(lastSeg.Duration.Seconds()))

		lines := []string{}
		lines = append(lines, "#EXTM3U")
		lines = append(lines, "#EXT-X-VERSION:3")
		lines = append(lines, fmt.Sprintf("#EXT-X-TARGETDURATION:%d", targetDur+1))
		lines = append(lines, fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d", firstSeg.MSN))
		lines = append(lines, fmt.Sprintf("#EXT-X-DISCONTINUITY-SEQUENCE:%d", firstSeg.MSN))
		lines = append(lines, "")

		for _, seg := range mediaSegs[startWith:] {
			lines = append(lines, "#EXT-X-DISCONTINUITY")
			lines = append(lines, fmt.Sprintf("#EXT-X-PROGRAM-DATE-TIME:%s", seg.Time.Format(time.RFC3339Nano)))
			lines = append(lines, fmt.Sprintf("#EXTINF:%f,", seg.Duration.Seconds()))
			if subOffsetMS != 0 {
				lines = append(lines, fmt.Sprintf("segment%05d.vtt?sub_offset_ms=%d", seg.MSN, subOffsetMS))
			} else {
				lines = append(lines, fmt.Sprintf("segment%05d.vtt", seg.MSN))
			}
		}
		lines = append(lines, "")
		playlist := []byte(strings.Join(lines, "\n"))
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		http.ServeContent(w, r, subFile, time.Now(), bytes.NewReader(playlist))
		return
	}

	if strings.HasSuffix(subFile, ".vtt") {
		// HLS WebVTT segments must have cue times relative to the segment.
		// We map the requested segment MSN to a time window based on the source rendition
		// segment durations, then filter transcript events into that window.
		var msn int
		if _, err := fmt.Sscanf(subFile, "segment%05d.vtt", &msn); err != nil {
			if _, err2 := fmt.Sscanf(subFile, "segment%d.vtt", &msn); err2 != nil {
				errors.WriteHTTPNotFound(w, "invalid subtitle segment", err)
				return
			}
		}

		rend, err := m3u8.GetRendition("source")
		if err != nil {
			errors.WriteHTTPNotFound(w, "could not get source rendition", err)
			return
		}

		rend.SegmentLock.RLock()
		mediaSegs := make([]*media.Segment, len(rend.Segments))
		copy(mediaSegs, rend.Segments)
		rend.SegmentLock.RUnlock()

		// Compute segment start/end in a synthetic media timeline where segs[0] starts at t=0.
		segIdx := -1
		for i, s := range mediaSegs {
			if int(s.MSN) == msn {
				segIdx = i
				break
			}
		}
		if segIdx == -1 {
			// Segment not in retention window.
			w.Header().Set("X-Streamplace-Transcript-Events", fmt.Sprintf("%d", len(a.MediaManager.GetTranscriptSegments(user))))
			w.Header().Set("Content-Type", "text/vtt")
			http.ServeContent(w, r, subFile, time.Now(), bytes.NewReader([]byte("WEBVTT\n\n")))
			return
		}

		segmentStartMS := mediaSegs[segIdx].StartMS
		segmentEndMS := segmentStartMS + mediaSegs[segIdx].Duration.Milliseconds()

		transcriptSegs := a.MediaManager.GetTranscriptSegments(user)
		// Apply a constant offset to subtitle timing by shifting the transcript window.
		// Positive offset delays subtitles; negative offset makes them earlier.
		vtt := aigateway.GenerateVTTForSegment(transcriptSegs, segmentStartMS-subOffsetMS, segmentEndMS-subOffsetMS)
		// HLS WebVTT cues are in LOCAL time (relative to the VTT). Players need a
		// mapping onto the media timeline. If we don't have a measured MPEGTS start
		// timestamp, derive it from our synthetic segment timeline (ms -> 90kHz ticks).
		mpegts := uint64(segmentStartMS * 90)
		mpegtsSource := "start_ms"
		if mediaSegs[segIdx].StartTS != nil {
			// StartTS is splitmuxsink running-time in nanoseconds. Convert to 90kHz ticks.
			mpegts = (*mediaSegs[segIdx].StartTS * 90) / 1_000_000
			mpegtsSource = "start_ns"
		}
		prefix := fmt.Sprintf("WEBVTT\n\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:%d\n\n", mpegts)
		if bytes.HasPrefix(vtt, []byte("WEBVTT\n\n")) {
			vtt = append([]byte(prefix), vtt[len([]byte("WEBVTT\n\n")):]...)
		} else {
			vtt = append([]byte(prefix), vtt...)
		}
		w.Header().Set("X-Streamplace-VTT-Segment-StartMS", fmt.Sprintf("%d", segmentStartMS))
		w.Header().Set("X-Streamplace-VTT-SubOffsetMS", fmt.Sprintf("%d", subOffsetMS))
		w.Header().Set("X-Streamplace-VTT-MPEGTS", fmt.Sprintf("%d", mpegts))
		w.Header().Set("X-Streamplace-VTT-MPEGTS-Source", mpegtsSource)
		w.Header().Set("X-Streamplace-Transcript-Events", fmt.Sprintf("%d", len(transcriptSegs)))
		w.Header().Set("Content-Type", "text/vtt")
		http.ServeContent(w, r, subFile, time.Now(), bytes.NewReader(vtt))
		return
	}

	errors.WriteHTTPNotFound(w, "subtitle file not found", nil)
}
