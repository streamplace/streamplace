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
)

func Combine(ctx context.Context, cli *config.CLI, debugDir string, outFile string, inputs []string) error {
	gstinit.InitGST()

	if debugDir != "" {
		err := os.MkdirAll(debugDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create debug directory: %w", err)
		}
	}
	log.Debug(context.Background(), "combine command: starting", "outFile", outFile, "inputs", inputs)
	ctx = log.WithDebugValue(ctx, cli.Debug)
	cryptoSigner, err := createSigner(ctx, cli)
	if err != nil {
		return err
	}
	ms, err := media.MakeMediaSigner(ctx, cli, "combine", cryptoSigner, nil)
	if err != nil {
		return err
	}

	log.Log(ctx, "combining segments", "outFile", outFile, "inputs", inputs)
	outFd, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer outFd.Close()
	inputFds := make([]io.ReadSeeker, len(inputs))
	for i, input := range inputs {
		fd, err := os.Open(input)
		if err != nil {
			return err
		}
		defer fd.Close()
		inputFds[i] = fd
	}
	err = media.CombineSegments(ctx, inputFds, ms, outFd)
	if err != nil {
		return err
	}
	err = CheckCombined(ctx, cli, outFd, debugDir)
	if err != nil {
		return err
	}
	return nil
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
