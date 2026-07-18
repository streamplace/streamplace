package renditions

import (
	"fmt"
	"sort"

	"stream.place/streamplace/pkg/placestream"
)

// BuildSourceRenditions names one source rendition per video track in a
// segment. The highest-resolution track keeps the well-known "source" name
// (default playback, thumbnails, and multistream egress all follow it); extra
// tracks become "source-<height>p" with a per-height collision counter suffix
// (e.g. dual 720p tracks become "source-720p" and "source-720p-2"). bitrate is
// the whole-segment bitrate, known only at this granularity here — per-track
// bitrates are measured downstream (livehls peakBitrate) — so it goes on the
// top track only.
func BuildSourceRenditions(spseg *placestream.Segment, bitrate int) []Rendition {
	sourceIdx := make([]int, len(spseg.Video))
	for i := range sourceIdx {
		sourceIdx[i] = i
	}
	sort.Slice(sourceIdx, func(a, b int) bool {
		va, vb := spseg.Video[sourceIdx[a]], spseg.Video[sourceIdx[b]]
		return va.Width*va.Height > vb.Width*vb.Height
	})
	sourceRenditions := []Rendition{}
	usedNames := map[string]int{}
	for rank, vi := range sourceIdx {
		v := spseg.Video[vi]
		name := "source"
		if rank != 0 {
			base := fmt.Sprintf("source-%dp", v.Height)
			name = base
			if count, used := usedNames[base]; used {
				name = fmt.Sprintf("%s-%d", base, count+1)
			}
		}
		usedNames[name] = usedNames[name] + 1
		r := Rendition{
			Name:   name,
			Width:  v.Width,
			Height: v.Height,
		}
		if rank == 0 {
			r.Bitrate = bitrate
		}
		sourceRenditions = append(sourceRenditions, r)
	}
	return sourceRenditions
}
