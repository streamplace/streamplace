//go:build !static

// This runs during dynamic builds in dev

// The streamplacedeps pkg-config file is what puts every bundled C and Rust
// library on the link line, the Rust crates that meson builds alongside it
// (iroh-streamplace and slatedb-store) included.
package streamplacedeps

// #cgo pkg-config: streamplacedeps
import "C"
