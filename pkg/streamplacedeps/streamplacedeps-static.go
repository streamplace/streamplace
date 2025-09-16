//go:build static

// This runs during static builds when we're making production executables

package streamplacedeps

// #cgo pkg-config: streamplacedeps-uninstalled
import "C"
