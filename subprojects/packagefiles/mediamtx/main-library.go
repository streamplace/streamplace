// entrypoint for library version of mediamtx
package main

import (
	"os"

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
