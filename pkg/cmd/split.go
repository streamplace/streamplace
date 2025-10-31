package cmd

import (
	"context"
	"os"
	"path/filepath"

	"stream.place/streamplace/pkg/media"
)

func Split(ctx context.Context, inFile, outDir string) error {
	bs, err := os.ReadFile(inFile)
	if err != nil {
		return err
	}
	outFiles, err := media.SplitSegments(ctx, bs)
	if err != nil {
		return err
	}
	for _, outFile := range outFiles {
		err = os.WriteFile(filepath.Join(outDir, outFile.Filename), outFile.Data, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
