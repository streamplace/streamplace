package media

import (
	"encoding/binary"
	"fmt"
	"time"
)

type cmafFragmentTiming struct {
	TrackID     uint32
	DecodeTime  uint64
	Duration    uint64
	SampleCount uint32
}

type cmafTrackFragmentMetadata struct {
	timing               cmafFragmentTiming
	firstSampleFlags     uint32
	haveFirstSampleFlags bool
}

type cmafTRUNMetadata struct {
	sampleCount          uint32
	duration             uint64
	firstSampleFlags     uint32
	haveFirstSampleFlags bool
}

type cmafTFHDMetadata struct {
	trackID               uint32
	defaultSampleDuration uint32
	defaultSampleFlags    uint32
	haveDefaultFlags      bool
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
	metadata, err := inspectCMAFTrackFragmentMetadata(data)
	if err != nil {
		return cmafFragmentTiming{}, err
	}
	return metadata.timing, nil
}

func inspectCMAFTrackFragmentMetadata(data []byte) (cmafTrackFragmentMetadata, error) {
	var metadata cmafTrackFragmentMetadata
	var defaults cmafTFHDMetadata
	var haveTrackID bool
	var haveDecodeTime bool
	var haveSamples bool
	var haveFirstSample bool
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		switch boxType {
		case "tfhd":
			parsed, err := parseCMAFTFHD(payload)
			if err != nil {
				return err
			}
			defaults = parsed
			metadata.timing.TrackID = parsed.trackID
			haveTrackID = true
		case "tfdt":
			decodeTime, err := parseCMAFTFDT(payload)
			if err != nil {
				return err
			}
			metadata.timing.DecodeTime = decodeTime
			haveDecodeTime = true
		case "trun":
			parsed, err := parseCMAFTRUNMetadata(payload, defaults)
			if err != nil {
				return err
			}
			metadata.timing.SampleCount += parsed.sampleCount
			metadata.timing.Duration += parsed.duration
			if !haveFirstSample && parsed.sampleCount > 0 {
				haveFirstSample = true
				metadata.firstSampleFlags = parsed.firstSampleFlags
				metadata.haveFirstSampleFlags = parsed.haveFirstSampleFlags
			}
			haveSamples = true
		}
		return nil
	})
	if err != nil {
		return cmafTrackFragmentMetadata{}, err
	}
	if !haveTrackID || !haveDecodeTime || !haveSamples {
		return cmafTrackFragmentMetadata{}, fmt.Errorf("CMAF track fragment is missing tfhd, tfdt, or trun")
	}
	if metadata.timing.SampleCount == 0 || metadata.timing.Duration == 0 {
		return cmafTrackFragmentMetadata{}, fmt.Errorf("CMAF track fragment has no sample duration")
	}
	return metadata, nil
}

func inspectCMAFFirstVideoSampleIndependent(data []byte, videoTrackID uint32) (bool, error) {
	return inspectCMAFFragmentIndependence(data, map[uint32]bool{videoTrackID: true})
}

func inspectCMAFFragmentHasVideo(data []byte, videoTrackIDs map[uint32]bool) (bool, error) {
	var foundVideo bool
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "moof" {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "traf" {
				return nil
			}
			metadata, err := inspectCMAFTrackFragmentMetadata(childPayload)
			if err != nil {
				return err
			}
			if videoTrackIDs[metadata.timing.TrackID] {
				foundVideo = true
			}
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	return foundVideo, nil
}

func inspectCMAFFragmentIndependence(data []byte, videoTrackIDs map[uint32]bool) (bool, error) {
	var foundVideo bool
	var independent bool
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "moof" {
			return nil
		}
		if foundVideo {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "traf" {
				return nil
			}
			metadata, err := inspectCMAFTrackFragmentMetadata(childPayload)
			if err != nil {
				return err
			}
			if !videoTrackIDs[metadata.timing.TrackID] {
				return nil
			}
			if foundVideo {
				return fmt.Errorf("CMAF moof contains multiple video track fragments")
			}
			foundVideo = true
			independent = metadata.haveFirstSampleFlags && isCMAFSyncSample(metadata.firstSampleFlags)
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	return independent, nil
}

func isCMAFSyncSample(flags uint32) bool {
	const (
		sampleDependsOnMask = 0x03000000
		sampleDoesNotDepend = 0x02000000
		sampleIsNonSync     = 0x00010000
	)
	return flags&sampleDependsOnMask == sampleDoesNotDepend && flags&sampleIsNonSync == 0
}

func cmafVideoTrackIDs(data []byte) (map[uint32]bool, error) {
	videoTrackIDs := make(map[uint32]bool)
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "moov" {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "trak" {
				return nil
			}
			trackID, handler, err := parseCMAFTrak(childPayload)
			if err != nil {
				return err
			}
			if handler == "vide" {
				videoTrackIDs[trackID] = true
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if len(videoTrackIDs) == 0 {
		return nil, fmt.Errorf("CMAF init contains no video track")
	}
	return videoTrackIDs, nil
}

func cmafTrackTimescale(data []byte) (uint32, error) {
	var timescale uint32
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "moov" {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "trak" {
				return nil
			}
			return walkCMAFBoxes(childPayload, func(trackChildType string, trackChildPayload []byte) error {
				if trackChildType != "mdia" {
					return nil
				}
				return walkCMAFBoxes(trackChildPayload, func(mediaChildType string, mediaChildPayload []byte) error {
					if mediaChildType != "mdhd" || timescale != 0 {
						return nil
					}
					parsed, err := parseCMAFMDHD(mediaChildPayload)
					if err == nil {
						timescale = parsed
					}
					return err
				})
			})
		})
	})
	if err != nil {
		return 0, err
	}
	if timescale == 0 {
		return 0, fmt.Errorf("CMAF init contains no track timescale")
	}
	return timescale, nil
}

func cmafAudioChannels(data []byte) (int, error) {
	var channels int
	var foundAudio bool
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "moov" {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "trak" {
				return nil
			}
			trackChannels, found, err := cmafAudioTrackChannels(childPayload)
			if err != nil {
				return err
			}
			if !found {
				return nil
			}
			if foundAudio {
				return fmt.Errorf("CMAF init contains multiple audio tracks")
			}
			channels = trackChannels
			foundAudio = true
			return nil
		})
	})
	if err != nil {
		return 0, err
	}
	if !foundAudio {
		return 0, fmt.Errorf("CMAF init contains no audio track")
	}
	return channels, nil
}

func cmafAudioTrackChannels(data []byte) (int, bool, error) {
	var handler string
	err := walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "mdia" {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "hdlr" {
				return nil
			}
			if len(childPayload) < 12 {
				return fmt.Errorf("hdlr is truncated")
			}
			handler = string(childPayload[8:12])
			return nil
		})
	})
	if err != nil {
		return 0, false, err
	}
	if handler != "soun" {
		return 0, false, nil
	}

	var channels int
	err = walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		if boxType != "mdia" {
			return nil
		}
		return walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
			if childType != "minf" {
				return nil
			}
			return walkCMAFBoxes(childPayload, func(minfChildType string, minfChildPayload []byte) error {
				if minfChildType != "stbl" {
					return nil
				}
				return walkCMAFBoxes(minfChildPayload, func(stblChildType string, stblChildPayload []byte) error {
					if stblChildType != "stsd" || channels != 0 {
						return nil
					}
					if len(stblChildPayload) < 8 {
						return fmt.Errorf("stsd is truncated")
					}
					return walkCMAFBoxes(stblChildPayload[8:], func(entryType string, entryPayload []byte) error {
						if entryType != "mp4a" || channels != 0 {
							return nil
						}
						if len(entryPayload) < 18 {
							return fmt.Errorf("mp4a sample entry is truncated")
						}
						channels = int(binary.BigEndian.Uint16(entryPayload[16:18]))
						if channels == 0 {
							return fmt.Errorf("mp4a sample entry has no channels")
						}
						return nil
					})
				})
			})
		})
	})
	if err != nil {
		return 0, false, err
	}
	if channels == 0 {
		return 0, false, fmt.Errorf("CMAF audio track contains no mp4a sample entry")
	}
	return channels, true, nil
}

func parseCMAFMDHD(payload []byte) (uint32, error) {
	if len(payload) < 4 {
		return 0, fmt.Errorf("mdhd is truncated")
	}
	switch payload[0] {
	case 0:
		if len(payload) < 16 {
			return 0, fmt.Errorf("version 0 mdhd is truncated")
		}
		return binary.BigEndian.Uint32(payload[12:16]), nil
	case 1:
		if len(payload) < 24 {
			return 0, fmt.Errorf("version 1 mdhd is truncated")
		}
		return binary.BigEndian.Uint32(payload[20:24]), nil
	default:
		return 0, fmt.Errorf("unsupported mdhd version %d", payload[0])
	}
}

func cmafDecodeTimeDuration(decodeTime uint64, timescale uint32) time.Duration {
	if timescale == 0 {
		return 0
	}
	wholeSeconds := decodeTime / uint64(timescale)
	remainder := decodeTime % uint64(timescale)
	return time.Duration(wholeSeconds)*time.Second + time.Duration(remainder)*time.Second/time.Duration(timescale)
}

func parseCMAFTrak(data []byte) (trackID uint32, handler string, err error) {
	var haveTrackID bool
	var haveHandler bool
	err = walkCMAFBoxes(data, func(boxType string, payload []byte) error {
		switch boxType {
		case "tkhd":
			trackID, err = parseCMAFTKHD(payload)
			if err == nil {
				haveTrackID = true
			}
		case "mdia":
			err = walkCMAFBoxes(payload, func(childType string, childPayload []byte) error {
				if childType != "hdlr" {
					return nil
				}
				if len(childPayload) < 12 {
					return fmt.Errorf("hdlr is truncated")
				}
				handler = string(childPayload[8:12])
				haveHandler = true
				return nil
			})
		}
		return err
	})
	if err != nil {
		return 0, "", err
	}
	if !haveTrackID || !haveHandler {
		return 0, "", fmt.Errorf("CMAF trak is missing tkhd or hdlr")
	}
	return trackID, handler, nil
}

func parseCMAFTKHD(payload []byte) (uint32, error) {
	if len(payload) < 4 {
		return 0, fmt.Errorf("tkhd is truncated")
	}
	switch payload[0] {
	case 0:
		if len(payload) < 16 {
			return 0, fmt.Errorf("version 0 tkhd is truncated")
		}
		return binary.BigEndian.Uint32(payload[12:16]), nil
	case 1:
		if len(payload) < 24 {
			return 0, fmt.Errorf("version 1 tkhd is truncated")
		}
		return binary.BigEndian.Uint32(payload[20:24]), nil
	default:
		return 0, fmt.Errorf("unsupported tkhd version %d", payload[0])
	}
}

func parseCMAFTFHD(payload []byte) (cmafTFHDMetadata, error) {
	if len(payload) < 8 {
		return cmafTFHDMetadata{}, fmt.Errorf("tfhd is truncated")
	}
	flags := binary.BigEndian.Uint32(payload[0:4]) & 0x00ffffff
	metadata := cmafTFHDMetadata{trackID: binary.BigEndian.Uint32(payload[4:8])}
	offset := 8
	if flags&0x000001 != 0 {
		if len(payload) < offset+8 {
			return cmafTFHDMetadata{}, fmt.Errorf("tfhd base-data-offset is truncated")
		}
		offset += 8
	}
	if flags&0x000002 != 0 {
		if len(payload) < offset+4 {
			return cmafTFHDMetadata{}, fmt.Errorf("tfhd sample-description-index is truncated")
		}
		offset += 4
	}
	if flags&0x000008 != 0 {
		if len(payload) < offset+4 {
			return cmafTFHDMetadata{}, fmt.Errorf("tfhd default sample duration is truncated")
		}
		metadata.defaultSampleDuration = binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4
	}
	if flags&0x000010 != 0 {
		if len(payload) < offset+4 {
			return cmafTFHDMetadata{}, fmt.Errorf("tfhd default sample size is truncated")
		}
		offset += 4
	}
	if flags&0x000020 != 0 {
		if len(payload) < offset+4 {
			return cmafTFHDMetadata{}, fmt.Errorf("tfhd default sample flags are truncated")
		}
		metadata.defaultSampleFlags = binary.BigEndian.Uint32(payload[offset : offset+4])
		metadata.haveDefaultFlags = true
	}
	return metadata, nil
}

func parseCMAFTRUNMetadata(payload []byte, defaults cmafTFHDMetadata) (cmafTRUNMetadata, error) {
	if len(payload) < 8 {
		return cmafTRUNMetadata{}, fmt.Errorf("trun is truncated")
	}
	flags := binary.BigEndian.Uint32(payload[0:4]) & 0x00ffffff
	metadata := cmafTRUNMetadata{sampleCount: binary.BigEndian.Uint32(payload[4:8])}
	offset := 8
	if flags&0x000001 != 0 {
		if len(payload) < offset+4 {
			return cmafTRUNMetadata{}, fmt.Errorf("trun data offset is truncated")
		}
		offset += 4
	}
	// GStreamer treats the combination of first-sample-flags and per-sample
	// flags as invalid and gives precedence to the per-sample flags.
	if flags&0x000004 != 0 && flags&0x000400 == 0 {
		if len(payload) < offset+4 {
			return cmafTRUNMetadata{}, fmt.Errorf("trun first-sample-flags is truncated")
		}
		metadata.firstSampleFlags = binary.BigEndian.Uint32(payload[offset : offset+4])
		metadata.haveFirstSampleFlags = metadata.sampleCount > 0
		offset += 4
	}
	for i := uint32(0); i < metadata.sampleCount; i++ {
		sampleDuration := defaults.defaultSampleDuration
		if flags&0x000100 != 0 {
			if len(payload) < offset+4 {
				return cmafTRUNMetadata{}, fmt.Errorf("trun sample duration is truncated at sample %d", i)
			}
			sampleDuration = binary.BigEndian.Uint32(payload[offset : offset+4])
			offset += 4
		} else if sampleDuration == 0 {
			return cmafTRUNMetadata{}, fmt.Errorf("trun has no sample duration at sample %d", i)
		}
		metadata.duration += uint64(sampleDuration)
		if flags&0x000200 != 0 {
			if len(payload) < offset+4 {
				return cmafTRUNMetadata{}, fmt.Errorf("trun sample size is truncated at sample %d", i)
			}
			offset += 4
		}
		if flags&0x000400 != 0 {
			if len(payload) < offset+4 {
				return cmafTRUNMetadata{}, fmt.Errorf("trun sample flags are truncated at sample %d", i)
			}
			if i == 0 && !metadata.haveFirstSampleFlags {
				metadata.firstSampleFlags = binary.BigEndian.Uint32(payload[offset : offset+4])
				metadata.haveFirstSampleFlags = true
			}
			offset += 4
		}
		if flags&0x000800 != 0 {
			if len(payload) < offset+4 {
				return cmafTRUNMetadata{}, fmt.Errorf("trun composition offset is truncated at sample %d", i)
			}
			offset += 4
		}
	}
	if !metadata.haveFirstSampleFlags && metadata.sampleCount > 0 && defaults.haveDefaultFlags {
		metadata.firstSampleFlags = defaults.defaultSampleFlags
		metadata.haveFirstSampleFlags = true
	}
	return metadata, nil
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
	metadata, err := parseCMAFTRUNMetadata(payload, cmafTFHDMetadata{defaultSampleDuration: defaultSampleDuration})
	if err != nil {
		return 0, 0, err
	}
	return metadata.sampleCount, metadata.duration, nil
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
