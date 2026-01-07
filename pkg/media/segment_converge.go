package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/Eyevinn/mp4ff/mp4"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

var MaxSegmentTries = 10

// run this segment through the segmenter/splitter until it comes out the
// same, meaning we can cleanly get it in and out of a concatenated mp4 file
func ConvergeSegment(ctx context.Context, cli *config.CLI, bs []byte, now int64, streamer string, doH264Parse bool) ([]byte, error) {
	cli.DumpDebugSegment(ctx, fmt.Sprintf("converge-segment-%s.mp4", streamer), bytes.NewReader(bs))

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
		if cli.SegmentDebugDir != "" {
			mydir := filepath.Join(cli.SegmentDebugDir, streamer)
			err := os.MkdirAll(mydir, 0755)
			if err != nil {
				return nil, fmt.Errorf("failed to create debug directory: %w", err)
			}
			aqt := aqtime.FromMillis(now)
			outFile := filepath.Join(cli.SegmentDebugDir, fmt.Sprintf("%s-attempt-%03d.mp4", aqt.FileSafeString(), i))
			err = os.WriteFile(outFile, currentBs, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to write debug file: %w", err)
			}
			log.Log(ctx, "wrote debug file", "path", outFile)
		}
		buf := bytes.Buffer{}
		err := CombineSegmentsUnsigned(ctx, []io.ReadSeeker{bytes.NewReader(currentBs)}, &buf, doH264Parse)
		if err != nil {
			return nil, fmt.Errorf("failed to attempt segment convergence: %w", err)
		}
		previousBs = currentBs
		currentBs = buf.Bytes()
		mp4file, err := mp4.DecodeFile(bytes.NewReader(currentBs))
		if err != nil {
			return nil, fmt.Errorf("failed to decode segment: %w", err)
		}
		btrt := mp4file.Moov.Trak.Mdia.Minf.Stbl.Stsd.AvcX.Btrt
		btrt.AvgBitrate = 0
		btrt.MaxBitrate = 0
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
