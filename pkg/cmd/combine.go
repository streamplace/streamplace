package cmd

import (
	"context"
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
	outBs, err := media.CombineSegments(ctx, inputs, ms)
	if err != nil {
		return err
	}
	fd, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fd.Write(outBs)
	if err != nil {
		return err
	}
	return nil
}
