package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
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
	repo, err := a.ATSync.SyncBlueskyRepoCached(ctx, user)
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
		answer, err := a.MediaManager.WebRTCPlayback2(ctx, user, rendition, &offer, "")
		if err != nil {
			errors.WriteHTTPInternalServerError(w, fmt.Sprintf("error playing back: %s", err.Error()), err)
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
			errors.WriteHTTPInternalServerError(w, fmt.Sprintf("error ingesting: %s", err.Error()), err)
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

// thumbnailMaxAge is how stale a thumbnail may be before we treat the user as
// offline and stop serving it. It must comfortably exceed thumbnailInterval (the
// rate at which live thumbnails are refreshed) to avoid flickering mid-stream.
const thumbnailMaxAge = 24 * time.Hour

func (a *StreamplaceAPI) HandleThumbnailPlayback(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx = log.WithLogValues(r.Context(), "func", "HandleThumbnailPlayback")
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
		fpath := a.CLI.ThumbnailFilePath(user)
		mt, ok := a.CLI.ThumbnailModTime(user)
		if !ok {
			errors.WriteHTTPNotFound(w, "thumbnail not found", nil)
			return
		}
		// A thumbnail that hasn't been refreshed recently means the user is no
		// longer live, so don't serve a stale preview. WideOpen (dev) skips this.
		if !a.CLI.WideOpen && time.Since(mt) > thumbnailMaxAge {
			errors.WriteHTTPNotFound(w, "no recent thumbnail", nil)
			return
		}
		log.Debug(ctx, "serving thumbnail", "fpath", fpath)
		http.ServeFile(w, r, fpath)
	}
}
