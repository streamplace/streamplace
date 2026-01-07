package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"time"

	"stream.place/streamplace/pkg/rtcrec"
)

func main() {
	err := Start()
	if err != nil {
		log.Fatal(err)
	}
}

func Start() error {
	if len(os.Args) == 1 {
		return fmt.Errorf("usage: rtcrec [decode|trim]")
	}
	if len(os.Args) > 1 && os.Args[1] == "decode" {
		return Decode()
	}
	if len(os.Args) > 1 && os.Args[1] == "trim" {
		return Trim()
	}
	return fmt.Errorf("unknown command: %s", os.Args[1])
}

func Trim() error {
	var startDuration time.Duration
	flag.DurationVar(&startDuration, "start", 0, "timestamp where we should start our clip")
	var endDuration time.Duration
	flag.DurationVar(&endDuration, "end", 0, "timestamp where we should end our clip")
	var inPath string
	flag.StringVar(&inPath, "in-path", "", "path to the file to decode")
	var outPath string
	flag.StringVar(&outPath, "out-path", "", "path to the file to write the trimmed file to")
	err := flag.CommandLine.Parse(os.Args[2:])
	if err != nil {
		return err
	}
	if startDuration == 0 && endDuration == 0 {
		return fmt.Errorf("start or end duration is required (otherwise, you know, the cp command is right there)")
	}
	if inPath == "" {
		return fmt.Errorf("in-path is required")
	}
	if outPath == "" {
		return fmt.Errorf("out-path is required")
	}
	inFile, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer inFile.Close()
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	dec, err := rtcrec.MakeWebRTCDecoder(inFile)
	if err != nil {
		return err
	}
	encoder, err := rtcrec.MakeWebRTCEncoder(outFile)
	if err != nil {
		return err
	}
	var startCutoff *time.Time
	var endCutoff *time.Time
	if startDuration == 0 {
		startCutoff = &time.Time{}
	}
	if endDuration == 0 {
		t := time.Unix(math.MaxInt64/2, 0) // i had it set to max but there were rollover issues
		endCutoff = &t
	}
	included := 0
	dropped := 0
	for {
		ev, err := dec.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if startCutoff == nil {
			t := ev.Time.Add(startDuration)
			startCutoff = &t
		}
		if endCutoff == nil {
			t := ev.Time.Add(endDuration)
			endCutoff = &t
		}
		// we only rewrite trackread events
		if ev.TrackRead == nil {
			if ev.Time.Before(*endCutoff) {
				// included++
				encoder.Event(*ev)
			}
			continue
		}
		// fmt.Printf("ev.Time: %+v, startCutoff: %+v, endCutoff: %+v\n", ev.Time, *startCutoff, *endCutoff)
		if ev.Time.Before(*startCutoff) {
			dropped++
			continue
		}
		included++
		ev.Time = ev.Time.Add(-startDuration)
		encoder.Event(*ev)
	}
	fmt.Printf("included: %d, dropped: %d\n", included, dropped)
	return nil
}

func Decode() error {
	var path string
	flag.StringVar(&path, "path", "", "path to the file to decode")
	err := flag.CommandLine.Parse(os.Args[2:])
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("path is required")
	}
	return DecodeFile(path)
}

func DecodeFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec, err := rtcrec.MakeWebRTCDecoder(f)
	if err != nil {
		return err
	}
	for {
		ev, err := dec.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if ev.TrackRead != nil {
			// spitting out the data as base64 is pointless, replace with a label
			n := len(ev.TrackRead.Data)
			byteString := fmt.Sprintf("%d bytes", n)
			bs, err := json.Marshal(ev)
			if err != nil {
				return err
			}
			var m map[string]any
			err = json.Unmarshal(bs, &m)
			if err != nil {
				return err
			}
			m["trackRead"].(map[string]any)["data"] = byteString
			bs, err = json.Marshal(m)
			if err != nil {
				return err
			}
			fmt.Println(string(bs))
		} else {
			bs, err := json.Marshal(ev)
			if err != nil {
				return err
			}
			fmt.Println(string(bs))
		}
	}
}
