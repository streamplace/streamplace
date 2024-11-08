package api

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aquareum.tv/aquareum/pkg/aqtime"
	"aquareum.tv/aquareum/pkg/errors"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"
)

func (a *AquareumAPI) NormalizeUser(user string) string {
	alias, ok := a.Aliases[user]
	if ok {
		user = alias
	}
	user = strings.ToLower(user)
	return user
}

func (a *AquareumAPI) HandleMP4Playback(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user = a.NormalizeUser(user)
		var delayMS int64 = 3000
		userDelay := r.URL.Query().Get("delayms")
		if userDelay != "" {
			var err error
			delayMS, err = strconv.ParseInt(userDelay, 10, 64)
			if err != nil {
				errors.WriteHTTPBadRequest(w, "error parsing delay", err)
				return
			}
			if delayMS > 10000 {
				errors.WriteHTTPBadRequest(w, "delay too large, maximum 10000", nil)
				return
			}
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(200)
		g, ctx := errgroup.WithContext(ctx)
		pr, pw := io.Pipe()
		bufw := bufio.NewWriter(pw)
		g.Go(func() error {
			return a.MediaManager.SegmentToMP4(ctx, user, bufw)
		})
		g.Go(func() error {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
			_, err := io.Copy(w, pr)
			return err
		})
		g.Wait()
	}
}

func (a *AquareumAPI) HandleMKVPlayback(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user = a.NormalizeUser(user)
		var delayMS int64 = 1000
		userDelay := r.URL.Query().Get("delayms")
		if userDelay != "" {
			var err error
			delayMS, err = strconv.ParseInt(userDelay, 10, 64)
			if err != nil {
				errors.WriteHTTPBadRequest(w, "error parsing delay", err)
				return
			}
			if delayMS > 10000 {
				errors.WriteHTTPBadRequest(w, "delay too large, maximum 10000", nil)
				return
			}
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(200)
		g, ctx := errgroup.WithContext(ctx)
		pr, pw := io.Pipe()
		bufw := bufio.NewWriter(pw)
		g.Go(func() error {
			return a.MediaManager.SegmentToMKVPlusOpus(ctx, user, bufw)
		})
		g.Go(func() error {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
			_, err := io.Copy(w, pr)
			return err
		})
		g.Wait()
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

func (a *AquareumAPI) HandleHLSPlayback(ctx context.Context) httprouter.Handle {
	return NoCache(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user = a.NormalizeUser(user)
		file := p.ByName("file")
		if file == "" {
			errors.WriteHTTPBadRequest(w, "file required", nil)
			return
		}
		m3u8, err := a.MediaManager.SegmentToHLSOnce(ctx, user)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "SegmentToHLSOnce failed", nil)
			return
		}
		buf, err := m3u8.GetSegment(file)
		if err != nil {
			errors.WriteHTTPNotFound(w, "segment not found", err)
			return
		}
		http.ServeContent(w, r, file, time.Now(), bytes.NewReader(buf))
	})
}

func (a *AquareumAPI) HandleThumbnailPlayback(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user = a.NormalizeUser(user)
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
