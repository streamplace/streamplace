package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

var MaxSegmentTries = 10

// run this segment through the segmenter/splitter until it comes out the
// same, meaning we can cleanly get it in and out of a concatenated mp4 file
func ConvergeSegment(ctx context.Context, cli *config.CLI, bs []byte, now int64, streamer string, doH264Parse bool) ([]byte, error) {
	log.Debug(ctx, "parsing segment media data", "size", len(bs))
	_, err := ParseSegmentMediaData(ctx, bs)
	if err != nil {
		return nil, fmt.Errorf("error parsing segment media data: %w", err)
	}
	// rewrite segmented audio timestamps to work around bug where the last
	// audio segment gets no duration and then gets dropped upon rewrite
	smearedBuf := &bytes.Buffer{}
	log.Debug(ctx, "rewriting audio timestamps", "size", len(bs))
	err = RewriteAudioTimestamps(ctx, cli, bytes.NewReader(bs), smearedBuf, false)
	if err != nil {
		return nil, fmt.Errorf("error rewriting audio timestamps: %w", err)
	}
	bs = smearedBuf.Bytes()
	log.Debug(ctx, "converging segment", "size", len(bs))

	previousBs := []byte{}
	currentBs := bs
	i := 0
	for i = 0; i <= MaxSegmentTries; i++ {
		if slices.Compare(previousBs, currentBs) == 0 {
			break
		}
		buf := bytes.Buffer{}
		err := CombineSegmentsUnsigned(ctx, []io.ReadSeeker{bytes.NewReader(currentBs)}, &buf, doH264Parse)
		if err != nil {
			// mp4mux sometimes fails transiently (e.g. "Could not multiplex stream");
			// treat that as a retryable convergence attempt rather than a fatal error.
			if strings.Contains(err.Error(), "Could not multiplex stream") {
				log.Warn(ctx, "transient mux error during convergence, retrying", "try", i, "error", err)
				continue
			}
			return nil, fmt.Errorf("failed to attempt segment convergence: %w", err)
		}
		previousBs = currentBs
		currentBs = buf.Bytes()
		mp4file, err := mp4.DecodeFile(bytes.NewReader(currentBs))
		if err != nil {
			return nil, fmt.Errorf("failed to decode segment: %w", err)
		}
		if mp4file != nil && mp4file.Moov != nil {
			if mp4file.Moov.Mvhd != nil {
				mp4file.Moov.Mvhd.CreationTime = 0
				mp4file.Moov.Mvhd.ModificationTime = 0
			}
			for _, trak := range mp4file.Moov.Traks {
				if trak == nil {
					continue
				}
				if trak.Tkhd != nil {
					trak.Tkhd.CreationTime = 0
					trak.Tkhd.ModificationTime = 0
				}
				if trak.Mdia != nil && trak.Mdia.Mdhd != nil {
					trak.Mdia.Mdhd.CreationTime = 0
					trak.Mdia.Mdhd.ModificationTime = 0
				}
			}

			if mp4file.Moov.Trak != nil && mp4file.Moov.Trak.Mdia != nil && mp4file.Moov.Trak.Mdia.Minf != nil && mp4file.Moov.Trak.Mdia.Minf.Stbl != nil && mp4file.Moov.Trak.Mdia.Minf.Stbl.Stsd != nil && mp4file.Moov.Trak.Mdia.Minf.Stbl.Stsd.AvcX != nil && mp4file.Moov.Trak.Mdia.Minf.Stbl.Stsd.AvcX.Btrt != nil {
				btrt := mp4file.Moov.Trak.Mdia.Minf.Stbl.Stsd.AvcX.Btrt
				btrt.AvgBitrate = 0
				btrt.MaxBitrate = 0
			}
		}
		// log.Log(ctx, "btrt", "average bitrate", btrt.AvgBitrate, "max bitrate", btrt.MaxBitrate)
		encodedBuf := bytes.Buffer{}
		err = mp4file.Encode(&encodedBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to encode segment: %w", err)
		}
		currentBs = encodedBuf.Bytes()
	}
	if slices.Compare(previousBs, currentBs) != 0 {
		return nil, fmt.Errorf("failed to converge segment after %d tries", MaxSegmentTries)
	}
	bs = currentBs
	log.Debug(ctx, "converged segments", "tries", i, "size", len(bs))
	return currentBs, nil
}
