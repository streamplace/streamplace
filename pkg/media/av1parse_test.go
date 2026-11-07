package media

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
)

// TestAV1ParsePassesEveryTU guards the av1parse patch
// (subprojects/packagefiles/gstreamer-full/av1parse-caps.patch): qtdemux
// refreshes the video caps when it reads the next fragment, and unpatched
// av1parse reset its parser on that caps event, dropping every TU up to the
// next sequence header. The 4K120 fixture loses the last 3 TUs of its first
// GoP that way.
func TestAV1ParsePassesEveryTU(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		tus     int
	}{
		{"av1-opus-frag.mp4", 60},
		{"av1-4k120-opus-frag.mp4", 240},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			gstinit.InitGST()
			launched, err := gst.ParseLaunch(strings.Join([]string{
				fmt.Sprintf("filesrc location=%s ! qtdemux name=d", getFixture(tc.fixture)),
				"d.video_0 ! tee name=t",
				"t. ! queue ! appsink name=raw sync=false",
				fmt.Sprintf("t. ! queue ! %s ! appsink name=parsed sync=false", VideoAV1.Parser),
				"d.audio_0 ! fakesink sync=false",
			}, "\n"))
			require.NoError(t, err)
			pipeline := launched.(gst.Pipeline)

			// size@pts of every buffer each sink receives
			var mu sync.Mutex
			got := map[string][]string{}
			for _, name := range []string{"raw", "parsed"} {
				pipeline.GetByName(name).(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
					NewSample: func(sink app.AppSink) gst.FlowReturn {
						sample := sink.PullSample()
						if sample == nil {
							return gst.FlowEOS
						}
						buffer := sample.GetBuffer()
						mu.Lock()
						got[name] = append(got[name], fmt.Sprintf("%d@%d", buffer.GetSize(), buffer.PTS()))
						mu.Unlock()
						return gst.FlowOK
					},
				})
			}

			ctx := context.Background()
			busErr := make(chan error, 1)
			go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
			require.NotEqual(t, gst.StateChangeFailure, pipeline.SetState(gst.StatePlaying))
			defer pipeline.SetState(gst.StateNull)
			require.NoError(t, <-busErr)

			require.Len(t, got["raw"], tc.tus)
			require.Equal(t, got["raw"], got["parsed"])
		})
	}
}
