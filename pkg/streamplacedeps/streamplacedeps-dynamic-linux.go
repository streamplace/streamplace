//go:build linux && !static

// This runs during dynamic builds in dev

package streamplacedeps

// #cgo pkg-config: ${SRCDIR}/../../build-linux-amd64/lib/pkgconfig/streamplacedeps.pc
// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR}/../../build-linux-amd64/lib
import "C"
