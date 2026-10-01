package vod

import (
	"sort"
	"strconv"
	"stream.place/streamplace/pkg/muxl"
)

func metafileTextTracks(meta *Metafile) []textProbeJSON {
	var tracks []textProbeJSON
	for id, t := range meta.Tracks {
		if t.Type == "text" {
			tracks = append(tracks, textProbeJSON{TrackID: id, Language: t.Language, Label: t.Label})
		}
	}
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].TrackID < tracks[j].TrackID })
	return tracks
}

func mergeCatalog(dst, src *muxl.MuxlCatalog) *muxl.MuxlCatalog {
	if dst == nil {
		return src
	}
	if src == nil {
		return dst
	}
	if src.Video != nil {
		if dst.Video == nil {
			dst.Video = src.Video
		} else {
			for k, v := range src.Video.Renditions {
				dst.Video.Renditions[k] = v
			}
		}
	}
	if src.Audio != nil {
		if dst.Audio == nil {
			dst.Audio = src.Audio
		} else {
			for k, v := range src.Audio.Renditions {
				dst.Audio.Renditions[k] = v
			}
		}
	}
	if src.Text != nil {
		if dst.Text == nil {
			dst.Text = src.Text
		} else {
			for k, v := range src.Text.Renditions {
				dst.Text.Renditions[k] = v
			}
		}
	}
	return dst
}

// catalogCaptionReference chooses an AV track actually present in this GoP.
// Keeping the association here avoids guessing it from byte ordering later.
func catalogCaptionReference(cat *muxl.MuxlCatalog, tracks map[string][]byte) (string, uint32) {
	if cat == nil {
		return "", 0
	}
	var id uint32
	var scale uint32
	if cat.Video != nil {
		for _, c := range cat.Video.Renditions {
			key := strconv.FormatUint(uint64(c.TrackID()), 10)
			if _, ok := tracks[key]; ok && (scale == 0 || c.TrackID() < id) {
				id = c.TrackID()
				scale = c.Timescale()
			}
		}
	}
	if scale == 0 && cat.Audio != nil {
		for _, c := range cat.Audio.Renditions {
			key := strconv.FormatUint(uint64(c.TrackID()), 10)
			if _, ok := tracks[key]; ok && (scale == 0 || c.TrackID() < id) {
				id = c.TrackID()
				scale = c.Timescale()
			}
		}
	}
	if scale == 0 {
		return "", 0
	}
	return strconv.FormatUint(uint64(id), 10), scale
}
