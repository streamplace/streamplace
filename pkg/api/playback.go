package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
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
		answer, err := a.MediaManager.WebRTCIngest(ctx, &offer, mediaSigner, pc, make(chan struct{}))
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

		if strings.HasSuffix(file, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		} else {
			if session != "" {
				spmetrics.SessionSeen(user, session)
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
