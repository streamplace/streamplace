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

func Combine(ctx context.Context, build *config.BuildFlags, allArgs []string) error {
	gstinit.InitGST()
	cli := &config.CLI{Build: build}

	var debugDir string
	// Simple flag parsing for debug-dir
	args := allArgs
	for i, arg := range allArgs {
		if arg == "--debug-dir" && i+1 < len(allArgs) {
			debugDir = allArgs[i+1]
			// Remove the flag from args
			args = append(allArgs[:i], allArgs[i+2:]...)
			break
		}
	}

	if debugDir != "" {
		err := os.MkdirAll(debugDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create debug directory: %w", err)
		}
	}
	log.Debug(context.Background(), "combine command: starting", "args", args)
	ctx = log.WithDebugValue(ctx, cli.Debug)
	cryptoSigner, err := createSigner(ctx, cli)
	if err != nil {
		return err
	}
	ms, err := media.MakeMediaSigner(ctx, cli, "combine", cryptoSigner, nil)
	if err != nil {
		return err
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: streamplace combine [--debug-dir dir] <output> <input1> [input2...]")
	}
	outFile := args[0]
	inputs := args[1:]
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
