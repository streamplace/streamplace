package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"stream.place/streamplace/pkg/rtcrec"
)

func main() {
	err := Start()
	if err != nil {
		log.Fatal(err)
	}
}

func Start() error {
	var path string
	flag.StringVar(&path, "path", "", "path to the file to decode")
	flag.Parse()
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
