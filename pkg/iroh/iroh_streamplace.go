package iroh_streamplace

import (
	_ "stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	_ "stream.place/streamplace/pkg/streamplacedeps"
)

// #cgo darwin LDFLAGS: -framework Security -framework SystemConfiguration
import "C"
