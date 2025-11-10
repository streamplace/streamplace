package cmd

import (
	"context"
	"io"
	"os"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
)

func Combine(ctx context.Context, cli *config.CLI, args []string) error {
	cryptoSigner, err := createSigner(ctx, cli)
	if err != nil {
		return err
	}
	ms, err := media.MakeMediaSigner(ctx, cli, "combine", cryptoSigner, nil)
	if err != nil {
		return err
	}
	outFile := args[0]
	inputs := args[1:]
	gstinit.InitGST()
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
	return nil
}
