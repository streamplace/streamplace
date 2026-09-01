package media

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/llhls"
)

const (
	cmafTestSyncSampleFlags    = 0x02000000
	cmafTestNonSyncSampleFlags = 0x01010000
)

func TestCMAFFirstVideoSampleIndependence(t *testing.T) {
	tests := []struct {
		name    string
		videoID uint32
		trafs   [][]byte
		want    bool
		wantErr string
	}{
		{
			name:    "tfhd default sync flags",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, cmafTestSyncSampleFlags, nil, nil)},
			want:    true,
		},
		{
			name:    "tfhd default non-sync flags",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, cmafTestNonSyncSampleFlags, nil, nil)},
			want:    false,
		},
		{
			name:    "trun first-sample override",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, cmafTestNonSyncSampleFlags, cmafTestFlagsPtr(cmafTestSyncSampleFlags), nil)},
			want:    true,
		},
		{
			name:    "per-sample non-sync override",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, cmafTestSyncSampleFlags, nil, []uint32{cmafTestNonSyncSampleFlags})},
			want:    false,
		},
		{
			name:    "per-sample flags override invalid first-sample-flags field",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, cmafTestNonSyncSampleFlags, cmafTestFlagsPtr(cmafTestNonSyncSampleFlags), []uint32{cmafTestSyncSampleFlags})},
			want:    true,
		},
		{
			name:    "audio traf does not establish independence",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(1, cmafTestSyncSampleFlags, nil, nil)},
			want:    false,
		},
		{
			name:    "unmapped video track is unknown",
			videoID: 9,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, cmafTestSyncSampleFlags, nil, nil)},
			want:    false,
		},
		{
			name:    "missing sample flags are unknown",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, 0, nil, nil)},
			want:    false,
		},
		{
			name:    "malformed sample flags are conservative",
			videoID: 2,
			trafs:   [][]byte{cmafTestIndependenceTraf(2, cmafTestSyncSampleFlags, nil, nil)[:len(cmafTestIndependenceTraf(2, cmafTestSyncSampleFlags, nil, nil))-1]},
			want:    false,
			wantErr: "invalid CMAF box size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragment := cmafTestBox("moof", cmafTestConcat(tt.trafs...))
			got, err := inspectCMAFFirstVideoSampleIndependent(fragment, tt.videoID)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("inspectCMAFFirstVideoSampleIndependent() error = %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("inspectCMAFFirstVideoSampleIndependent() error = %v, want substring %q", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("inspectCMAFFirstVideoSampleIndependent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCMAFFirstVideoSampleIndependenceFindsVideoAmongMultipleTraf(t *testing.T) {
	fragment := cmafTestBox("moof", append(
		cmafTestIndependenceTraf(1, cmafTestNonSyncSampleFlags, nil, nil),
		cmafTestIndependenceTraf(2, cmafTestSyncSampleFlags, nil, nil)...,
	))

	got, err := inspectCMAFFirstVideoSampleIndependent(fragment, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("video traf was not recognized as independently decodable")
	}
}

func TestCMAFFirstVideoSampleIndependenceUsesInitHandlerMapping(t *testing.T) {
	init := cmafTestBox("ftyp", nil)
	init = append(init, cmafTestBox("moov", cmafTestConcat(
		cmafTestTrak(1, "soun"),
		cmafTestTrak(2, "vide"),
	))...)
	videoTrackIDs, err := cmafVideoTrackIDs(init)
	if err != nil {
		t.Fatal(err)
	}
	fragment := cmafTestBox("moof", cmafTestConcat(
		cmafTestIndependenceTraf(1, cmafTestSyncSampleFlags, nil, nil),
		cmafTestIndependenceTraf(2, cmafTestNonSyncSampleFlags, nil, nil),
	))

	got, err := inspectCMAFFragmentIndependence(fragment, videoTrackIDs)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("audio sync flags incorrectly established video independence")
	}
}

func TestCMAFInitMapsVideoTrackIDs(t *testing.T) {
	videoTrak := cmafTestTrak(2, "vide")
	audioTrak := cmafTestTrak(1, "soun")
	init := cmafTestBox("ftyp", nil)
	init = append(init, cmafTestBox("moov", cmafTestConcat(videoTrak, audioTrak))...)

	got, err := cmafVideoTrackIDs(init)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[2] {
		t.Fatalf("video track IDs = %#v, want only track 2", got)
	}
}

func TestCMAFInitVideoTrackMappingIsConservativeOnMalformedInput(t *testing.T) {
	_, err := cmafVideoTrackIDs(cmafTestBox("ftyp", nil))
	if err == nil || !strings.Contains(err.Error(), "contains no video track") {
		t.Fatalf("cmafVideoTrackIDs() error = %v, want missing-video error", err)
	}
}

func TestISOFMP4MuxUsesVideoSampleFlagsForIndependence(t *testing.T) {
	gstinit.InitGST()
	if gst.Find("isofmp4mux") == nil || gst.Find("x264enc") == nil {
		t.Skip("static GStreamer build with isofmp4mux and x264enc is required")
	}
	pipeline, err := gst.NewPipelineFromString(
		"isofmp4mux name=mux fragment-duration=2000000000 chunk-duration=1000000000 send-force-keyunit=false ! appsink name=sink sync=false\n" +
			"videotestsrc num-buffers=180 is-live=true pattern=ball ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency speed-preset=ultrafast bframes=0 key-int-max=45 ! h264parse ! video/x-h264,stream-format=avc,alignment=au ! mux.",
	)
	if err != nil {
		t.Fatal(err)
	}
	sinkElement, err := pipeline.GetElementByName("sink")
	if err != nil {
		t.Fatal(err)
	}
	state := &cmafTrackSink{
		ctx:          context.Background(),
		presentation: "test",
		track:        "video",
		window:       llhls.NewWindow(),
		partDuration: time.Second,
	}
	sink := app.SinkFromElement(sinkElement)
	sink.SetBufferListSupport(true)
	callbackErr := make(chan error, 1)
	done := make(chan struct{})
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			if err := state.sample(sample); err != nil {
				callbackErr <- err
				return gst.FlowError
			}
			return gst.FlowOK
		},
		EOSFunc: func(*app.Sink) { close(done) },
	})
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = pipeline.SetState(gst.StateNull)
		t.Fatal("timed out waiting for irregular-GOP isofmp4mux EOS")
	}
	_ = pipeline.SetState(gst.StateNull)
	select {
	case err := <-callbackErr:
		t.Fatal(err)
	default:
	}

	snapshot := state.window.Snapshot("test", "video")
	if len(snapshot.Init) == 0 || len(state.videoTrackIDs) == 0 {
		t.Fatalf("muxed init did not establish a video track mapping: init=%d tracks=%v", len(snapshot.Init), state.videoTrackIDs)
	}
	foundIndependent := false
	for _, segment := range snapshot.Segments {
		for _, part := range segment.Parts {
			want, err := inspectCMAFFragmentIndependence(part.Data, state.videoTrackIDs)
			if err != nil {
				t.Fatalf("part %d/%d independence inspection failed: %v", segment.MSN, part.Index, err)
			}
			if part.Independent != want {
				t.Fatalf("part %d/%d Independent=%v, parsed=%v", segment.MSN, part.Index, part.Independent, want)
			}
			foundIndependent = foundIndependent || part.Independent
		}
	}
	if !foundIndependent {
		t.Fatal("real isofmp4mux output contained no independently decodable video part")
	}
}

func cmafTestIndependenceTraf(trackID, defaultFlags uint32, firstFlags *uint32, sampleFlags []uint32) []byte {
	tfhdPayload := make([]byte, 16)
	binary.BigEndian.PutUint32(tfhdPayload[0:4], 0x000028)
	binary.BigEndian.PutUint32(tfhdPayload[4:8], trackID)
	binary.BigEndian.PutUint32(tfhdPayload[12:16], defaultFlags)
	tfhd := cmafTestBox("tfhd", tfhdPayload)
	tfdt := cmafTestTFDT(0, 0)
	flags := uint32(0x000100)
	if firstFlags != nil {
		flags |= 0x000004
	}
	if len(sampleFlags) > 0 {
		flags |= 0x000400
	}
	trunPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(trunPayload[0:4], flags)
	binary.BigEndian.PutUint32(trunPayload[4:8], 1)
	if firstFlags != nil && len(sampleFlags) == 0 {
		value := make([]byte, 4)
		binary.BigEndian.PutUint32(value, *firstFlags)
		trunPayload = append(trunPayload, value...)
	}
	trunPayload = append(trunPayload, []byte{0, 0, 0, 1}...)
	if len(sampleFlags) > 0 {
		value := make([]byte, 4)
		binary.BigEndian.PutUint32(value, sampleFlags[0])
		trunPayload = append(trunPayload, value...)
	}
	return cmafTestBox("traf", append(tfhd, append(tfdt, cmafTestBox("trun", trunPayload)...)...))
}

func cmafTestTrak(trackID uint32, handler string) []byte {
	tkhdPayload := make([]byte, 20)
	binary.BigEndian.PutUint32(tkhdPayload[0:4], 0x00000007)
	binary.BigEndian.PutUint32(tkhdPayload[12:16], trackID)
	tkhd := cmafTestBox("tkhd", tkhdPayload)
	hdlrPayload := make([]byte, 12)
	copy(hdlrPayload[8:12], handler)
	hdlr := cmafTestBox("hdlr", hdlrPayload)
	mdia := cmafTestBox("mdia", hdlr)
	return cmafTestBox("trak", append(tkhd, mdia...))
}

func cmafTestConcat(boxes ...[]byte) []byte {
	var data []byte
	for _, box := range boxes {
		data = append(data, box...)
	}
	return data
}

func cmafTestFlagsPtr(flags uint32) *uint32 {
	return &flags
}
