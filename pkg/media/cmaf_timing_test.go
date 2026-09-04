package media

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCMAFProgramDateTimeUsesFragmentDecodeTime(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	state := &cmafTrackSink{
		programDateTimeBase: base,
		timescale:           48_000,
	}

	got := state.fragmentProgramDateTime(cmafFragmentTiming{DecodeTime: 125_000}, time.Second)
	want := base.Add(125_000 * time.Second / 48_000)
	if !got.Equal(want) {
		t.Fatalf("fragment program date time = %s, want %s", got, want)
	}
}

func TestCMAFTrackTimescale(t *testing.T) {
	for _, tt := range []struct {
		name      string
		version   byte
		timescale uint32
	}{
		{name: "version 0", version: 0, timescale: 48_000},
		{name: "version 1", version: 1, timescale: 90_000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payloadSize := 16
			if tt.version == 1 {
				payloadSize = 24
			}
			payload := make([]byte, payloadSize)
			payload[0] = tt.version
			binary.BigEndian.PutUint32(payload[payloadSize-4:], tt.timescale)
			init := cmafTestBox("moov", cmafTestBox("trak", cmafTestBox("mdia", cmafTestBox("mdhd", payload))))

			got, err := cmafTrackTimescale(init)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.timescale {
				t.Fatalf("track timescale = %d, want %d", got, tt.timescale)
			}
		})
	}
}

func TestCMAFAudioChannels(t *testing.T) {
	for _, channels := range []uint16{1, 2} {
		t.Run(fmt.Sprintf("%d channels", channels), func(t *testing.T) {
			sampleEntry := make([]byte, 18)
			binary.BigEndian.PutUint16(sampleEntry[16:18], channels)
			stsdPayload := append(make([]byte, 8), cmafTestBox("mp4a", sampleEntry)...)
			hdlrPayload := append(make([]byte, 8), []byte("soun")...)
			mediaPayload := append(cmafTestBox("hdlr", hdlrPayload), cmafTestBox("minf", cmafTestBox("stbl", cmafTestBox("stsd", stsdPayload)))...)
			init := cmafTestBox("moov", cmafTestBox("trak", cmafTestBox("mdia", mediaPayload)))

			got, err := cmafAudioChannels(init)
			if err != nil {
				t.Fatal(err)
			}
			if got != int(channels) {
				t.Fatalf("audio channels = %d, want %d", got, channels)
			}
		})
	}
}

func TestInspectCMAFFragmentReadsTrackTiming(t *testing.T) {
	tfhd := cmafTestTFHD(2, 9000)
	tfdt := cmafTestTFDT(1, 0x00000000000f0000)
	trunPayload := make([]byte, 8+2*16)
	binary.BigEndian.PutUint32(trunPayload[0:4], 0x000f00)
	binary.BigEndian.PutUint32(trunPayload[4:8], 2)
	for i, sample := range [][4]int64{{9000, 100, 0, 12}, {10000, 110, 0, -8}} {
		offset := 8 + i*16
		for j, value := range sample {
			binary.BigEndian.PutUint32(trunPayload[offset+j*4:offset+(j+1)*4], uint32(value))
		}
	}
	trun := cmafTestBox("trun", trunPayload)
	fragment := cmafTestBox("moof", cmafTestBox("traf", append(tfhd, append(tfdt, trun...)...)))

	timings, err := inspectCMAFFragment(fragment)
	if err != nil {
		t.Fatal(err)
	}
	if len(timings) != 1 {
		t.Fatalf("expected one track timing, got %d", len(timings))
	}
	got := timings[0]
	if got.TrackID != 2 || got.DecodeTime != 0x00000000000f0000 || got.Duration != 19000 || got.SampleCount != 2 {
		t.Fatalf("unexpected track timing: %+v", got)
	}
}

func TestInspectCMAFFragmentUsesDefaultSampleDuration(t *testing.T) {
	tfhd := cmafTestTFHD(7, 1024)
	tfdt := cmafTestTFDT(0, 123)
	trunPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(trunPayload[4:8], 3)
	trun := cmafTestBox("trun", trunPayload)
	fragment := cmafTestBox("moof", cmafTestBox("traf", append(tfhd, append(tfdt, trun...)...)))

	timings, err := inspectCMAFFragment(fragment)
	if err != nil {
		t.Fatal(err)
	}
	if got := timings[0]; got.DecodeTime != 123 || got.Duration != 3072 || got.SampleCount != 3 {
		t.Fatalf("unexpected default-duration timing: %+v", got)
	}
}

func TestInspectCMAFFragmentRejectsMalformedTiming(t *testing.T) {
	tests := []struct {
		name     string
		fragment []byte
		want     string
	}{
		{
			name:     "missing track fragment",
			fragment: cmafTestBox("moof", nil),
			want:     "contains no track fragments",
		},
		{
			name: "truncated tfhd optional field",
			fragment: cmafTestBox("moof", cmafTestBox("traf", func() []byte {
				payload := make([]byte, 8)
				binary.BigEndian.PutUint32(payload[0:4], 0x000001)
				binary.BigEndian.PutUint32(payload[4:8], 1)
				return cmafTestBox("tfhd", payload)
			}())),
			want: "tfhd base-data-offset is truncated",
		},
		{
			name: "truncated sample data",
			fragment: cmafTestBox("moof", cmafTestBox("traf", func() []byte {
				tfhd := cmafTestTFHD(1, 1024)
				tfdt := cmafTestTFDT(0, 0)
				trunPayload := make([]byte, 8)
				binary.BigEndian.PutUint32(trunPayload[0:4], 0x000100)
				binary.BigEndian.PutUint32(trunPayload[4:8], 1)
				return append(tfhd, append(tfdt, cmafTestBox("trun", trunPayload)...)...)
			}())),
			want: "trun sample duration is truncated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := inspectCMAFFragment(tt.fragment)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("inspectCMAFFragment() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func cmafTestBox(boxType string, payload []byte) []byte {
	data := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(data[0:4], uint32(len(data)))
	copy(data[4:8], boxType)
	copy(data[8:], payload)
	return data
}

func cmafTestTFHD(trackID, defaultSampleDuration uint32) []byte {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], 0x000008)
	binary.BigEndian.PutUint32(payload[4:8], trackID)
	binary.BigEndian.PutUint32(payload[8:12], defaultSampleDuration)
	return cmafTestBox("tfhd", payload)
}

func cmafTestTFDT(version byte, decodeTime uint64) []byte {
	if version == 0 {
		payload := make([]byte, 8)
		binary.BigEndian.PutUint32(payload[4:8], uint32(decodeTime))
		return cmafTestBox("tfdt", payload)
	}
	payload := make([]byte, 12)
	payload[0] = version
	binary.BigEndian.PutUint64(payload[4:12], decodeTime)
	return cmafTestBox("tfdt", payload)
}
