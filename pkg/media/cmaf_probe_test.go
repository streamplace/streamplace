package media

import (
	"testing"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/gstinit"
)

// Dynamic developer environments may not have the Rust plugin
func TestCMAFMuxFactoryAvailable(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("cmafmux") == nil {
		t.Skip("cmafmux is unavailable")
	}
}
