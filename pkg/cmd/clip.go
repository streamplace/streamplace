package cmd

import (
	"context"
	"fmt"
	"os"

	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func Clip(ctx context.Context, args []string, out string) error {
	if out == "" {
		return fmt.Errorf("out is required")
	}
	log.Log(ctx, "clip", "out", out)
	gstinit.InitGST()
	fd, err := os.Create(out)
	if err != nil {
		return err
	}
	defer fd.Close()
	err = media.Clip(ctx, args, fd)
	if err != nil {
		return err
	}
	return nil
}
