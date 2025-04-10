package main

import (
	"context"
	"strconv"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"

	"stream.place/streamplace/pkg/cmd"
	// _ "github.com/go-gst/go-glib/glib"
	// _ "github.com/go-gst/go-gst/gst"
)

//#cgo pkg-config: streamplacedeps-uninstalled
import "C"

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
		log.Log(context.Background(), "exited uncleanly", "error", err)
	}
}

func main() {
	StreamplaceMain()
}
