package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/llhls"
	"stream.place/streamplace/pkg/log"
)

const llhlsBlockingRequestTimeout = 10 * time.Second

func (a *StreamplaceAPI) llWindow(r *http.Request, user string) (*llhls.Window, string, error) {
	did, err := a.NormalizeUser(r.Context(), user)
	if err != nil {
		return nil, "", err
	}
	window := a.MediaManager.GetLLWindow(did)
	if window == nil {
		return nil, did, nil
	}
	return window, did, nil
}

func (a *StreamplaceAPI) HandleLLHLSMaster(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		window, did, err := a.llWindow(r, p.ByName("user"))
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "invalid user", err)
			return
		}
		if window == nil {
			http.Redirect(w, r, "/xrpc/place.stream.playback.getLivePlaylist?streamer="+url.QueryEscape(p.ByName("user")), http.StatusTemporaryRedirect)
			return
		}
		presentation := window.Presentation()
		if presentation == "" {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/xrpc/place.stream.playback.getLivePlaylist?streamer="+url.QueryEscape(p.ByName("user")), http.StatusTemporaryRedirect)
			return
		}
		videoConfig := window.VideoConfig()
		if videoConfig.FrameRate != 0 && !validLLHLSFrameRate(videoConfig.FrameRate) {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/xrpc/place.stream.playback.getLivePlaylist?streamer="+url.QueryEscape(p.ByName("user")), http.StatusTemporaryRedirect)
			return
		}
		waitCtx, cancel := context.WithTimeout(r.Context(), llhlsBlockingRequestTimeout)
		waitErr := window.WaitForMaster(waitCtx, presentation)
		cancel()
		if waitErr != nil {
			if errors.Is(waitErr, context.Canceled) {
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			if errors.Is(waitErr, llhls.ErrReloadUnavailable) {
				http.Error(w, "LL-HLS presentation changed while loading master", http.StatusServiceUnavailable)
			} else if errors.Is(waitErr, llhls.ErrWaiterLimit) {
				http.Error(w, "too many LL-HLS master requests", http.StatusServiceUnavailable)
			} else {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "LL-HLS master metadata is not ready", http.StatusServiceUnavailable)
			}
			return
		}
		videoConfig = window.VideoConfig()
		audioConfig := window.AudioConfig()
		base := fmt.Sprintf("/api/playback/%s/llhls/%s", url.PathEscape(did), url.PathEscape(presentation))
		body, err := renderLLHLSMaster(base, videoConfig, audioConfig)
		if err != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "LL-HLS master metadata is not ready", http.StatusServiceUnavailable)
			return
		}
		writeLLHLSPlaylist(w, body)
	}
}

func renderLLHLSMaster(base string, videoConfig llhls.VideoConfig, audioConfig llhls.AudioConfig) (string, error) {
	if !validLLHLSFrameRate(videoConfig.FrameRate) {
		return "", fmt.Errorf("invalid LL-HLS video frame rate: %v", videoConfig.FrameRate)
	}
	if videoConfig.Bandwidth <= 0 || videoConfig.AverageBandwidth <= 0 || audioConfig.Bandwidth <= 0 || audioConfig.AverageBandwidth <= 0 {
		return "", errors.New("LL-HLS bandwidth metadata is not ready")
	}
	codec := videoConfig.Codec
	if codec == "" {
		codec = "avc1.64001f"
	}
	channels := audioConfig.Channels
	if channels <= 0 {
		channels = 2
	}
	videoBandwidth := videoConfig.Bandwidth
	videoAverageBandwidth := videoConfig.AverageBandwidth
	audioBandwidth := audioConfig.Bandwidth
	audioAverageBandwidth := audioConfig.AverageBandwidth
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:10\n")
	fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=%q,NAME=%q,DEFAULT=YES,AUTOSELECT=YES,CHANNELS=%q,CODECS=%q,URI=%q\n",
		"audio", "default", strconv.Itoa(channels), "mp4a.40.2", base+"/audio/index.m3u8")
	streamInf := fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,AVERAGE-BANDWIDTH=%d,CODECS=%q", sumBitrates(videoBandwidth, audioBandwidth), sumBitrates(videoAverageBandwidth, audioAverageBandwidth), codec+",mp4a.40.2")
	if videoConfig.Width > 0 && videoConfig.Height > 0 {
		streamInf += fmt.Sprintf(",RESOLUTION=%dx%d", videoConfig.Width, videoConfig.Height)
	}
	if validLLHLSFrameRate(videoConfig.FrameRate) {
		streamInf += fmt.Sprintf(",FRAME-RATE=%.3f", videoConfig.FrameRate)
	}
	streamInf += ",AUDIO=\"audio\",CLOSED-CAPTIONS=NONE"
	b.WriteString(streamInf + "\n" + base + "/video/index.m3u8\n")
	return b.String(), nil
}

func sumBitrates(values ...int) int {
	maxInt := int(^uint(0) >> 1)
	sum := 0
	for _, value := range values {
		if value <= 0 || sum > maxInt-value {
			return maxInt
		}
		sum += value
	}
	return sum
}

func (a *StreamplaceAPI) HandleLLHLS(ctx context.Context) httprouter.Handle {
	master := a.HandleLLHLSMaster(ctx)
	playlist := a.HandleLLHLSPlaylist(ctx)
	initSegment := a.HandleLLHLSInit(ctx)
	part := a.HandleLLHLSPart(ctx)
	segment := a.HandleLLHLSSegment(ctx)
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		path := strings.TrimPrefix(p.ByName("path"), "/")
		parts := strings.Split(path, "/")
		params := httprouter.Params{{Key: "user", Value: p.ByName("user")}}
		switch {
		case path == "master.m3u8":
			master(w, r, params)
		case len(parts) == 3 && parts[2] == "index.m3u8":
			params = append(params,
				httprouter.Param{Key: "presentation", Value: parts[0]},
				httprouter.Param{Key: "track", Value: parts[1]},
			)
			playlist(w, r, params)
		case len(parts) == 3 && parts[2] == "init.mp4":
			params = append(params,
				httprouter.Param{Key: "presentation", Value: parts[0]},
				httprouter.Param{Key: "track", Value: parts[1]},
			)
			initSegment(w, r, params)
		case len(parts) == 4 && strings.HasSuffix(parts[3], ".m4s"):
			params = append(params,
				httprouter.Param{Key: "presentation", Value: parts[0]},
				httprouter.Param{Key: "track", Value: parts[1]},
				httprouter.Param{Key: "msn", Value: parts[2]},
				httprouter.Param{Key: "part.m4s", Value: parts[3]},
			)
			part(w, r, params)
		case len(parts) == 3 && strings.HasSuffix(parts[2], ".m4s"):
			params = append(params,
				httprouter.Param{Key: "presentation", Value: parts[0]},
				httprouter.Param{Key: "track", Value: parts[1]},
				httprouter.Param{Key: "msn.m4s", Value: parts[2]},
			)
			segment(w, r, params)
		default:
			apierrors.WriteHTTPNotFound(w, "invalid LL-HLS path", nil)
		}
	}
}

func (a *StreamplaceAPI) HandleLLHLSPlaylist(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		window, did, err := a.llWindow(r, p.ByName("user"))
		if err != nil || window == nil {
			apierrors.WriteHTTPNotFound(w, "stream not live", err)
			return
		}
		presentation, track := p.ByName("presentation"), p.ByName("track")
		if presentation != window.Presentation() || (track != "video" && track != "audio") {
			apierrors.WriteHTTPNotFound(w, "track not found", nil)
			return
		}
		msn, part, reload, err := llhlsReloadQuery(r)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "invalid LL-HLS reload query", err)
			return
		}
		log.Debug(r.Context(), "LL-HLS playlist request", "presentation", presentation, "track", track, "msn", msn, "part", part, "blocking", reload, "skip", r.URL.Query().Get("_HLS_skip"))
		if reload {
			waitCtx, cancel := context.WithTimeout(r.Context(), llhlsBlockingRequestTimeout)
			err := window.Wait(waitCtx, presentation, track, msn, part)
			cancel()
			if err != nil {
				log.Debug(r.Context(), "LL-HLS playlist wait ended", "presentation", presentation, "track", track, "msn", msn, "part", part, "error", err)
				if errors.Is(err, llhls.ErrReloadUnavailable) {
					w.Header().Set("Cache-Control", "no-store")
					apierrors.WriteHTTPBadRequest(w, "invalid LL-HLS reload point", err)
				} else if errors.Is(err, context.DeadlineExceeded) {
					w.Header().Set("Cache-Control", "no-store")
					http.Error(w, "LL-HLS playlist reload timed out", http.StatusServiceUnavailable)
				} else if errors.Is(err, llhls.ErrWaiterLimit) {
					w.Header().Set("Cache-Control", "no-store")
					http.Error(w, "too many LL-HLS playlist reloads", http.StatusServiceUnavailable)
				} else {
					w.Header().Set("Cache-Control", "no-store")
					http.Error(w, "LL-HLS playlist reload failed", http.StatusServiceUnavailable)
				}
				return
			}
		}
		base := fmt.Sprintf("/api/playback/%s/llhls/%s/%s", url.PathEscape(did), url.PathEscape(presentation), track)
		renditionBase := fmt.Sprintf("/api/playback/%s/llhls/%s", url.PathEscape(did), url.PathEscape(presentation))
		body := window.Playlist(presentation, track,
			func(msn uint64, part uint32) string { return fmt.Sprintf("%s/%d/%d.m4s", base, msn, part) },
			func(msn uint64) string { return fmt.Sprintf("%s/%d.m4s", base, msn) },
			base+"/init.mp4",
			func(otherTrack string) string { return fmt.Sprintf("%s/%s/index.m3u8", renditionBase, otherTrack) })
		if body == "" {
			apierrors.WriteHTTPNotFound(w, "track not found", nil)
			return
		}
		log.Debug(r.Context(), "LL-HLS playlist response", "presentation", presentation, "track", track, "msn", msn, "part", part, "blocking", reload, "bytes", len(body))
		writeLLHLSPlaylist(w, body)
	}
}

func llhlsReloadQuery(r *http.Request) (msn uint64, part uint32, reload bool, err error) {
	values := r.URL.Query()
	msnValues, hasMSN := values["_HLS_msn"]
	partValues, hasPart := values["_HLS_part"]
	if !hasMSN {
		if hasPart {
			return 0, 0, false, fmt.Errorf("_HLS_part requires _HLS_msn")
		}
		return 0, 0, false, nil
	}
	if len(msnValues) != 1 || msnValues[0] == "" {
		return 0, 0, false, fmt.Errorf("_HLS_msn must be a single non-negative integer")
	}
	msn, err = strconv.ParseUint(msnValues[0], 10, 64)
	if err != nil {
		return 0, 0, false, fmt.Errorf("_HLS_msn must be a non-negative integer: %w", err)
	}
	if !hasPart {
		return msn, 0, true, nil
	}
	if len(partValues) != 1 || partValues[0] == "" {
		return 0, 0, false, fmt.Errorf("_HLS_part must be a single non-negative integer")
	}
	part64, parseErr := strconv.ParseUint(partValues[0], 10, 32)
	if parseErr != nil {
		return 0, 0, false, fmt.Errorf("_HLS_part must be a non-negative integer: %w", parseErr)
	}
	return msn, uint32(part64), true, nil
}

func (a *StreamplaceAPI) HandleLLHLSInit(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		window, _, err := a.llWindow(r, p.ByName("user"))
		if err != nil || window == nil {
			apierrors.WriteHTTPNotFound(w, "stream not live", err)
			return
		}
		presentation, track := p.ByName("presentation"), p.ByName("track")
		data := window.Snapshot(presentation, track).Init
		if len(data) == 0 {
			log.Warn(r.Context(), "LL-HLS init unavailable", "presentation", presentation, "track", track)
		} else {
			log.Debug(r.Context(), "LL-HLS init response", "presentation", presentation, "track", track, "bytes", len(data))
		}
		serveLLHLSBytes(w, r, data, "init.mp4", false, llhlsTrackMIMEType(track))
	}
}

func (a *StreamplaceAPI) HandleLLHLSPart(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		window, _, err := a.llWindow(r, p.ByName("user"))
		if err != nil || window == nil {
			apierrors.WriteHTTPNotFound(w, "stream not live", err)
			return
		}
		msn, e1 := strconv.ParseUint(p.ByName("msn"), 10, 64)
		part, e2 := strconv.ParseUint(strings.TrimSuffix(p.ByName("part.m4s"), ".m4s"), 10, 32)
		if e1 != nil || e2 != nil {
			apierrors.WriteHTTPBadRequest(w, "invalid media sequence or part", nil)
			return
		}
		presentation, track := p.ByName("presentation"), p.ByName("track")
		partIndex := uint32(part)
		waitCtx, cancel := context.WithTimeout(r.Context(), llhlsBlockingRequestTimeout)
		waitErr := window.WaitForPart(waitCtx, presentation, track, msn, partIndex)
		cancel()
		if errors.Is(waitErr, llhls.ErrPartUnavailable) {
			w.Header().Set("Cache-Control", "no-store")
			apierrors.WriteHTTPNotFound(w, "media not found", waitErr)
			return
		}
		if waitErr != nil {
			log.Debug(r.Context(), "LL-HLS part wait ended", "presentation", presentation, "track", track, "msn", msn, "part", partIndex, "error", waitErr)
			w.Header().Set("Cache-Control", "no-store")
			switch {
			case errors.Is(waitErr, context.Canceled):
				return
			case errors.Is(waitErr, llhls.ErrReloadUnavailable):
				apierrors.WriteHTTPNotFound(w, "LL-HLS presentation changed", waitErr)
			case errors.Is(waitErr, context.DeadlineExceeded):
				http.Error(w, "LL-HLS part request timed out", http.StatusServiceUnavailable)
			case errors.Is(waitErr, llhls.ErrWaiterLimit):
				http.Error(w, "too many LL-HLS part requests", http.StatusServiceUnavailable)
			default:
				http.Error(w, "LL-HLS part request failed", http.StatusServiceUnavailable)
			}
			return
		}
		data := window.Data(presentation, track, msn, partIndex)
		if len(data) == 0 {
			log.Warn(r.Context(), "LL-HLS part unavailable", "presentation", presentation, "track", track, "msn", msn, "part", partIndex)
		} else {
			log.Debug(r.Context(), "LL-HLS part response", "presentation", presentation, "track", track, "msn", msn, "part", partIndex, "bytes", len(data))
		}
		serveLLHLSBytes(w, r, data, "part.m4s", true, llhlsTrackMIMEType(track))
	}
}

func validLLHLSFrameRate(fps float64) bool {
	// Apple HLS authoring rules cap advertised video frame rates at 60 fps.
	return fps > 0 && fps <= 60 && !math.IsNaN(fps) && !math.IsInf(fps, 0)
}

func (a *StreamplaceAPI) HandleLLHLSSegment(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		window, _, err := a.llWindow(r, p.ByName("user"))
		if err != nil || window == nil {
			apierrors.WriteHTTPNotFound(w, "stream not live", err)
			return
		}
		msn, err := strconv.ParseUint(strings.TrimSuffix(p.ByName("msn.m4s"), ".m4s"), 10, 64)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "invalid media sequence", err)
			return
		}
		presentation, track := p.ByName("presentation"), p.ByName("track")
		data := window.SegmentData(presentation, track, msn)
		if len(data) == 0 {
			log.Warn(r.Context(), "LL-HLS segment unavailable", "presentation", presentation, "track", track, "msn", msn)
		} else {
			log.Debug(r.Context(), "LL-HLS segment response", "presentation", presentation, "track", track, "msn", msn, "bytes", len(data))
		}
		serveLLHLSBytes(w, r, data, "segment.m4s", true, llhlsTrackMIMEType(track))
	}
}

func writeLLHLSPlaylist(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func llhlsTrackMIMEType(track string) string {
	if track == "audio" {
		return "audio/mp4"
	}
	return "video/mp4"
}

func serveLLHLSBytes(w http.ResponseWriter, r *http.Request, data []byte, name string, immutable bool, contentType string) {
	if len(data) == 0 {
		w.Header().Set("Cache-Control", "no-store")
		apierrors.WriteHTTPNotFound(w, "media not found", nil)
		return
	}
	w.Header().Set("Content-Type", contentType)
	if immutable {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
