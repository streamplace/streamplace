// entrypoint for aquareum + mediamtx
package main

import (
	"os"

	"context"
	"strconv"

	"aquareum.tv/aquareum/pkg/cmd"
	"aquareum.tv/aquareum/pkg/config"
	"aquareum.tv/aquareum/pkg/log"
	"github.com/bluenviron/mediamtx/internal/core"
)

import "C"

//export MTXMain
func MTXMain() {
	s, ok := core.New(os.Args[2:])
	if !ok {
		os.Exit(1)
	}
	s.Wait()
}

//export AquareumMain
func AquareumMain() {
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
