//go:build !static

// This runs during dynamic builds in dev

package streamplacedeps

// #cgo pkg-config: streamplacedeps
import "C"
