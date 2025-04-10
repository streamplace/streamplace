package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/go-gst/go-gst/gst"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/media"
	_ "stream.place/streamplace/pkg/media/mediatesting"
)

func getFile() []byte {
	// Open input file
	inputFile, err := os.Open(media.GetFixture("5sec.mp4"))
	if err != nil {
		panic(err)
	}
	defer inputFile.Close()
	data, err := io.ReadAll(inputFile)
	if err != nil {
		panic(err)
	}
	return data
}

func Test(data []byte) {
	// Create buffers for output
	videoBuf := bytes.Buffer{}
	audioBuf := bytes.Buffer{}

	// Split MP4 into MPEG-TS video and MP4 audio
	// start := time.Now()
	err := media.MP4ToMPEGTSVideoMP4Audio(context.Background(), bytes.NewReader(data), &videoBuf, &audioBuf)
	if err != nil {
		panic(err)
	}
	// elapsed := time.Since(start)
	// require.Less(t, elapsed, 4*time.Second, "MP4 to MPEG-TS/MP4 conversion should take less than 4 seconds")

	// Verify outputs
	// require.Greater(t, videoBuf.Len(), 0, "Video buffer should not be empty")
	// require.Greater(t, audioBuf.Len(), 0, "Audio buffer should not be empty")

	// Join video and audio back together
	buf := bytes.Buffer{}
	// start = time.Now()
	err = media.MPEGTSVideoMP4AudioToMP4(context.Background(), &videoBuf, &audioBuf, &buf)
	if err != nil {
		panic(err)
	}
	// require.Greater(t, buf.Len(), 0, "Output buffer should not be empty")
	// elapsed = time.Since(start)
	// require.Less(t, elapsed, 4*time.Second, "MPEG-TS/MP4 to MP4 conversion should take less than 4 seconds")
	// after := getThreadCount(t, "Current thread count after test")
}

func main() {
	fmt.Println("Starting")
	errgroup, _ := errgroup.WithContext(context.Background())
	stats := &runtime.MemStats{}
	go func() {
		for {
			select {
			case <-time.Tick(time.Second):
				runtime.ReadMemStats(stats)
				inUse := stats.Sys - stats.HeapReleased
				fmt.Printf("Memory in use: %.2f MB (%.2f GB)\n", float64(inUse)/1024/1024, float64(inUse)/1024/1024/1024)
			}
		}
	}()
	data := getFile()
	gst.Init(nil)

	count := 0
	countChan := make(chan int)
	go func() {
		for {
			select {
			case <-countChan:
				count++
				if count > 0 && count%10 == 0 {
					fmt.Printf("Completed %d tests\n", count)
				}
			}
		}
	}()

	errgroup.SetLimit(10)
	for i := 0; i < 1000; i++ {
		errgroup.Go(func() error {
			Test(data)
			countChan <- 1
			return nil
		})
	}
	fmt.Println("Done")
	errgroup.Wait()
	runtime.GC()
	select {}
}
