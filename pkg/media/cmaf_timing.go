package media

import (
	"encoding/binary"
	"fmt"
)

type cmafFragmentTiming struct {
	TrackID     uint32
	DecodeTime  uint64
	Duration    uint64
	SampleCount uint32
}

func inspectCMAFFragment(data []byte) ([]cmafFragmentTiming, error) {
	var timings []cmafFragmentTiming
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "moof" {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "traf" {
				return nil
			}
			timing, err := inspectCMAFTrackFragment(childPayload)
			if err != nil {
				return err
			}
			timings = append(timings, timing)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if len(timings) == 0 {
		return nil, fmt.Errorf("CMAF fragment contains no track fragments")
	}
	return timings, nil
}

func inspectCMAFTrackFragment(data []byte) (cmafFragmentTiming, error) {
	var timing cmafFragmentTiming
	var defaultSampleDuration uint32
	haveTrackID := false
	haveDecodeTime := false
	haveSamples := false
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		switch boxType {
		case "tfhd":
			trackID, sampleDuration, err := parseCMAFTFHD(payload)
			if err != nil {
				return err
			}
			timing.TrackID = trackID
			defaultSampleDuration = sampleDuration
			haveTrackID = true
		case "tfdt":
			decodeTime, err := parseCMAFTFDT(payload)
			if err != nil {
				return err
			}
			timing.DecodeTime = decodeTime
			haveDecodeTime = true
		case "trun":
			sampleCount, duration, err := parseCMAFTRUN(payload, defaultSampleDuration)
			if err != nil {
				return err
			}
			timing.SampleCount += sampleCount
			timing.Duration += duration
			haveSamples = true
		}
		return nil
	})
	if err != nil {
		return cmafFragmentTiming{}, err
	}
	if !haveTrackID || !haveDecodeTime || !haveSamples {
		return cmafFragmentTiming{}, fmt.Errorf("CMAF track fragment is missing tfhd, tfdt, or trun")
	}
	if timing.SampleCount == 0 || timing.Duration == 0 {
		return cmafFragmentTiming{}, fmt.Errorf("CMAF track fragment has no sample duration")
	}
	return timing, nil
}

func parseCMAFTFHD(payload []byte) (trackID uint32, defaultSampleDuration uint32, err error) {
	if len(payload) < 8 {
		return 0, 0, fmt.Errorf("tfhd is truncated")
	}
	flags := binary.BigEndian.Uint32(payload[0:4]) & 0x00ffffff
	trackID = binary.BigEndian.Uint32(payload[4:8])
	offset := 8
	if flags&0x000001 != 0 {
		if len(payload) < offset+8 {
			return 0, 0, fmt.Errorf("tfhd base-data-offset is truncated")
		}
		offset += 8
	}
	if flags&0x000002 != 0 {
		if len(payload) < offset+4 {
			return 0, 0, fmt.Errorf("tfhd sample-description-index is truncated")
		}
		offset += 4
	}
	if flags&0x000008 != 0 {
		if len(payload) < offset+4 {
			return 0, 0, fmt.Errorf("tfhd default sample duration is truncated")
		}
		defaultSampleDuration = binary.BigEndian.Uint32(payload[offset : offset+4])
	}
	return trackID, defaultSampleDuration, nil
}

func parseCMAFTFDT(payload []byte) (uint64, error) {
	if len(payload) < 8 {
		return 0, fmt.Errorf("tfdt is truncated")
	}
	switch payload[0] {
	case 0:
		return uint64(binary.BigEndian.Uint32(payload[4:8])), nil
	case 1:
		if len(payload) < 12 {
			return 0, fmt.Errorf("64-bit tfdt is truncated")
		}
		return binary.BigEndian.Uint64(payload[4:12]), nil
	default:
		return 0, fmt.Errorf("unsupported tfdt version %d", payload[0])
	}
}

func parseCMAFTRUN(payload []byte, defaultSampleDuration uint32) (sampleCount uint32, duration uint64, err error) {
	if len(payload) < 8 {
		return 0, 0, fmt.Errorf("trun is truncated")
	}
	version := payload[0]
	flags := binary.BigEndian.Uint32(payload[0:4]) & 0x00ffffff
	sampleCount = binary.BigEndian.Uint32(payload[4:8])
	offset := 8
	if flags&0x000001 != 0 {
		if len(payload) < offset+4 {
			return 0, 0, fmt.Errorf("trun data offset is truncated")
		}
		offset += 4
	}
	if flags&0x000004 != 0 {
		if len(payload) < offset+4 {
			return 0, 0, fmt.Errorf("trun first-sample-flags is truncated")
		}
		offset += 4
	}
	for i := uint32(0); i < sampleCount; i++ {
		sampleDuration := defaultSampleDuration
		if flags&0x000100 != 0 {
			if len(payload) < offset+4 {
				return 0, 0, fmt.Errorf("trun sample duration is truncated at sample %d", i)
			}
			sampleDuration = binary.BigEndian.Uint32(payload[offset : offset+4])
			offset += 4
		} else if sampleDuration == 0 {
			return 0, 0, fmt.Errorf("trun has no sample duration at sample %d", i)
		}
		duration += uint64(sampleDuration)
		if flags&0x000200 != 0 {
			if len(payload) < offset+4 {
				return 0, 0, fmt.Errorf("trun sample size is truncated at sample %d", i)
			}
			offset += 4
		}
		if flags&0x000400 != 0 {
			if len(payload) < offset+4 {
				return 0, 0, fmt.Errorf("trun sample flags are truncated at sample %d", i)
			}
			offset += 4
		}
		if flags&0x000800 != 0 {
			if len(payload) < offset+4 {
				return 0, 0, fmt.Errorf("trun composition offset is truncated at sample %d", i)
			}
			if version == 0 {
				_ = binary.BigEndian.Uint32(payload[offset : offset+4])
			} else {
				_ = int32(binary.BigEndian.Uint32(payload[offset : offset+4]))
			}
			offset += 4
		}
	}
	if len(payload) < offset {
		return 0, 0, fmt.Errorf("trun sample data is truncated")
	}
	return sampleCount, duration, nil
}

func walkCMAFBoxes(data []byte, visit func(string, []byte) error) error {
	for len(data) > 0 {
		if len(data) < 8 {
			return fmt.Errorf("CMAF box header is truncated")
		}
		size := uint64(binary.BigEndian.Uint32(data[0:4]))
		headerSize := uint64(8)
		if size == 1 {
			if len(data) < 16 {
				return fmt.Errorf("CMAF extended box size is truncated")
			}
			size = binary.BigEndian.Uint64(data[8:16])
			headerSize = 16
		} else if size == 0 {
			size = uint64(len(data))
		}
		if size < headerSize || size > uint64(len(data)) {
			return fmt.Errorf("invalid CMAF box size %d", size)
		}
		if err := visit(string(data[4:8]), data[headerSize:size]); err != nil {
			return err
		}
		data = data[size:]
	}
	return nil
}
