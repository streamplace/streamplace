package iroh_streamplace

// This file exists because Go tests seem to be linking the tests in the wrong order
// doing it like this makes sure -lm is linked before the Rust code that needs it

// #cgo LDFLAGS: -lm
import "C"
