package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func Split(ctx context.Context, inFile, outDir string) error {
	inFD, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer inFD.Close()
	err = os.MkdirAll(outDir, 0755)
	if err != nil {
		return err
	}

	names := []string{}

	err = media.SplitSegments(ctx, inFD, func(fname string) media.ReadWriteSeekCloser {
		fullPath := filepath.Join(outDir, fname)
		names = append(names, fullPath)
		log.Log(ctx, "creating segment file", "path", fullPath)
		fd, err := os.Create(fullPath)
		if err != nil {
			log.Error(ctx, "failed to open segment file", "error", err)
			return nil
		}
		return fd
	})
	if err != nil {
		return fmt.Errorf("failed to split segments: %w", err)
	}

	for _, name := range names {
		fd, err := os.Open(name)
		if err != nil {
			return fmt.Errorf("failed to open segment file: %w", err)
		}
		defer fd.Close()
		bs, err := io.ReadAll(fd)
		if err != nil {
			return fmt.Errorf("failed to read segment file: %w", err)
		}
		_, err = media.ValidateMP4Media(ctx, bs)
		if err != nil {
			return fmt.Errorf("failed to validate segment file: %w", err)
		}
		log.Log(ctx, "validated segment file", "path", name)
	}

	return nil
}
