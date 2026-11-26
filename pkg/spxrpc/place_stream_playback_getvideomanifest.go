package spxrpc

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/linking"
	"stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamPlaybackGetVideoManifest(ctx context.Context, end int, sid string, start int, uri string) (*placestream.PlaybackGetVideoManifest_Output, error) {
	return nil, stubMisrouted("getVideoManifest")
}

func manifestSources(advertised, did, rkey, sid string, cdn bool) []placestream.PlaybackGetVideoManifest_Source {
	httpSource := placestream.PlaybackGetVideoManifest_Source{Type: "http"}
	sources := []placestream.PlaybackGetVideoManifest_Source{}
	if advertised != "" {
		if u, err := url.Parse(advertised); err == nil && u.Host != "" {
			fingerprint := u.Query().Get("certhash")
			endpoint := "https://" + u.Host
			broadcast := "vod/" + did + "/" + rkey + "/" + sid
			source := placestream.PlaybackGetVideoManifest_Source{Type: "moq", Url: &endpoint, Broadcast: &broadcast}
			if fingerprint != "" {
				source.Fingerprint = &fingerprint
			}
			sources = append(sources, source)
		}
	}
	if cdn {
		return append([]placestream.PlaybackGetVideoManifest_Source{httpSource}, sources...)
	}
	return append(sources, httpSource)
}

func buildVideoManifest(playback *videoPlayback, inits map[string]string, cdn vodCDN, advertised string) *placestream.PlaybackGetVideoManifest_Output {
	meta := playback.meta
	videos := videoTrackIDs(meta)
	audio := mainAudioIDs(meta, tracksOfType(meta, "audio"))
	defaultAudio := pickDefaultAudio(meta, audio)
	for i, id := range audio {
		if id == defaultAudio {
			copy(audio[1:i+1], audio[:i])
			audio[0] = id
			break
		}
	}
	ids := append(videos, audio...)
	out := &placestream.PlaybackGetVideoManifest_Output{Uri: playback.uri, Sid: playback.session.SID, Tracks: make([]placestream.PlaybackGetVideoManifest_Track, 0, len(ids))}
	for _, id := range ids {
		track := meta.Tracks[id]
		segments, _ := filterSegments(track.Segments, track.Timescale, playback.start, playback.end)
		bandwidth, _ := computeBandwidth(track)
		var totalTicks uint64
		for _, segment := range track.Segments {
			totalTicks += segment.DurationTicks
		}
		expires := cdn.tokenExpiry(ticksToDuration(totalTicks, track.Timescale))
		t := placestream.PlaybackGetVideoManifest_Track{Id: id, Type: track.Type, Codec: track.Codec, Bandwidth: int64(bandwidth), Timescale: int64(track.Timescale), Init: inits[id], Url: blobURL(cdn, playback.did, track.BlobCID, playback.session.SID, expires), Offsets: make([]int64, 0, len(segments)), Sizes: make([]int64, 0, len(segments)), Durations: make([]int64, 0, len(segments))}
		if track.Type == "video" {
			width, height := int64(track.Width), int64(track.Height)
			t.Width, t.Height = &width, &height
		} else {
			channels, rate := int64(channelsOrDefault(track.Channels)), int64(track.SampleRate)
			t.Channels, t.SampleRate = &channels, &rate
		}
		if len(segments) > 0 {
			for i, segment := range track.Segments {
				if segment.Offset == segments[0].Offset {
					t.First = int64(i)
					break
				}
			}
		}
		var ticks uint64
		for _, segment := range segments {
			t.Offsets = append(t.Offsets, meta.FlatHeaderSize+segment.Offset)
			t.Sizes = append(t.Sizes, segment.Size)
			t.Durations = append(t.Durations, int64(segment.DurationTicks))
			ticks += segment.DurationTicks
		}
		duration := ticksToDuration(ticks, track.Timescale).Milliseconds()
		if duration > out.DurationMs {
			out.DurationMs = duration
		}
		out.Tracks = append(out.Tracks, t)
	}
	rkey := playback.uri[strings.LastIndexByte(playback.uri, '/')+1:]
	out.Sources = manifestSources(advertised, playback.did, rkey, playback.session.SID, cdn.URL != "")
	return out
}

func (s *Server) videoManifest(c echo.Context) (*videoPlayback, *vodParts, *placestream.PlaybackGetVideoManifest_Output, error) {
	playback, err := s.resolvePlaybackRequest(c, false)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err := s.videoParts(c.Request().Context(), playback.resolved.blobCID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.loadInits(c.Request().Context(), parts); err != nil {
		return nil, nil, nil, err
	}
	advertised := ""
	if s.cli != nil && s.cli.VODMoQ && s.MoqURL != nil {
		advertised = s.MoqURL()
	}
	manifest := buildVideoManifest(playback, parts.inits, s.vodCDN(), advertised)
	return playback, parts, manifest, nil
}

func (s *Server) HandleGetVideoManifest(c echo.Context) error {
	playback, _, manifest, err := s.videoManifest(c)
	if err != nil {
		return err
	}
	s.logManifestRequest(c, playback.uri, playback.session.ID, "", "manifest")
	// It carries this viewer's session: a shared cache must not hand it to others.
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, manifest)
}

func (s *Server) VideoBootPayload(req *http.Request, uri string, hasTime bool) (*linking.VODBootPayload, error) {
	request := req.Clone(req.Context())
	copiedURL := *request.URL
	request.URL = &copiedURL
	request.URL.RawQuery = url.Values{"uri": {uri}}.Encode()
	// Reuse the node's IP extractor so HTML boot and XRPC bind clients identically.
	c := s.e.NewContext(request, nil)
	playback, parts, manifest, err := s.videoManifest(c)
	if err != nil {
		return nil, err
	}
	payload := &linking.VODBootPayload{Manifest: manifest}
	if starterAllowed(playback, hasTime) {
		payload.Starter, err = s.loadStarter(request.Context(), parts)
		if err != nil {
			return nil, err
		}
	}
	s.logManifestRequest(c, playback.uri, playback.session.ID, "", "manifest")
	if payload.Starter != nil {
		logBytes := func(id string, values ...*string) {
			track := playback.meta.Tracks[id]
			if len(track.Segments) == 0 {
				return
			}
			var size int64
			for _, value := range values {
				if value != nil {
					size += int64(base64.StdEncoding.DecodedLen(len(*value)) - strings.Count((*value)[max(0, len(*value)-2):], "="))
				}
			}
			segment := track.Segments[0]
			if size > segment.Size {
				size = segment.Size
			}
			if size > 0 {
				offset := playback.meta.FlatHeaderSize + segment.Offset
				s.logSegmentRequest(c, track.BlobCID, playback.did, playback.session.ID, offset, offset+size-1)
			}
		}
		logBytes(payload.Starter.Video.Track, payload.Starter.Video.Key, payload.Starter.Video.Rest)
		if payload.Starter.Audio != nil {
			logBytes(payload.Starter.Audio.Track, payload.Starter.Audio.Data)
		}
	}
	return payload, nil
}
