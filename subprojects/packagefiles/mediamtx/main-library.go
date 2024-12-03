// entrypoint for library version of mediamtx
package main

import (
	"os"

	"aquareum.tv/aquareum/pkg/cmd"
	"aquareum.tv/aquareum/pkg/config"
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
	cmd.Start(&config.BuildFlags{
		Version:   "foo",
		BuildTime: 0,
		UUID:      "bar",
	})
}
