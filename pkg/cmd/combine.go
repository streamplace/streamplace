package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"stream.place/streamplace/pkg/aqio"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
)

func Combine(ctx context.Context, cli *config.CLI, debugDir string, outFile string, inputs []string) error {
	gstinit.InitGST()

	if debugDir != "" {
		err := os.MkdirAll(debugDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create debug directory: %w", err)
		}
	}
	ctx = log.WithDebugValue(ctx, cli.Debug)
	log.Log(ctx, "combining segments", "outFile", outFile, "inputs", inputs)

	outFd, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer outFd.Close()

	readers := make([]io.Reader, 0, len(inputs))
	for _, input := range inputs {
		fd, err := os.Open(input)
		if err != nil {
			return err
		}
		defer fd.Close()
		readers = append(readers, fd)
	}

	// Inputs are canonical MUXL segments; concatenated they wrap straight into
	// a flat MP4 — one synthesized ftyp+moov over every segment, each segment's
	// signature preserved verbatim. No remux, no re-signing.
	if err := muxl.RunMuxlWrap(ctx, io.MultiReader(readers...), "flat", outFd); err != nil {
		return fmt.Errorf("failed to combine segments: %w", err)
	}

	return CheckCombined(ctx, cli, outFd, debugDir)
}

func CheckCombined(ctx context.Context, cli *config.CLI, inFD io.ReadWriteSeeker, debugDir string) error {
	_, err := inFD.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = media.SplitSegments(ctx, cli, inFD, func(fname string) media.ReadWriteSeekCloser {
		if debugDir == "" {
			return aqio.NewReadWriteSeeker([]byte{})
		}
		fd, err := os.Create(filepath.Join(debugDir, fname))
		if err != nil {
			panic(fmt.Errorf("failed to create debug file: %w", err))
		}
		log.Log(ctx, "created debug file", "path", filepath.Join(debugDir, fname))
		return fd
	})
	if err != nil {
		return err
	}
	return nil
}
