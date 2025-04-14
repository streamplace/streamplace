package main

import (
	"context"
	"errors"
	"os"
	"strconv"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"

	"stream.place/streamplace/pkg/cmd"
	// _ "github.com/go-gst/go-glib/glib"
	// _ "github.com/go-gst/go-gst/gst"
)

//#cgo pkg-config: streamplacedeps-uninstalled
import "C"

var cleanExits = []error{
	cmd.ErrCaughtSignal,
	context.DeadlineExceeded,
}

//export StreamplaceMain
func StreamplaceMain() {
	i, err := strconv.ParseInt(BuildTime, 10, 64)
	if err != nil {
		panic(err)
	}
	err = cmd.Start(&config.BuildFlags{
		Version:   Version,
		BuildTime: i,
		UUID:      UUID,
	})
	if err != nil {
		for _, e := range cleanExits {
			if errors.Is(err, e) {
				log.Log(context.Background(), "exited cleanly", "error", err)
				os.Exit(0)
			}
		}
		log.Log(context.Background(), "exited uncleanly", "error", err)
		os.Exit(1)
	}
}

func main() {
	StreamplaceMain()
}
