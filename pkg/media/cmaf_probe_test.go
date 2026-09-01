package media

import (
	"testing"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/gstinit"
)

// Dynamic developer environments may not have the Rust plugin.
func TestISOFMP4MuxFactoryAvailable(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil {
		t.Skip("isofmp4mux is unavailable")
	}
}
