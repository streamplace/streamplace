package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/test/remote"
)

var thumbnailTestCases = []struct {
	name      string
	fixtureFn func() string
}{
	{
		name: "SampleSegment",
		fixtureFn: func() string {
			return getFixture("sample-segment.mp4")
		},
	},
	{
		name: "MuxlSegment",
		fixtureFn: func() string {
			return remote.RemoteFixture("c6b57a53fc5a2234dbdd388922f0e293d8063d2b30620321e974b7c85640f228/2026-03-17T19-02-08-607Z-muxl_segment_input.fmp4")
		},
	},
	{
		name: "MuxlSegment10Sec",
		fixtureFn: func() string {
			return remote.RemoteFixture("82d20ee62b02f1c3a727b3001f1fa939afb757f9f205fa438d7b5753e1253eef/2026-04-11T22-39-41-861Z-packetize-input-019d7eb3-6f24-776c-ba1b-2f909a2379d7.mp4")
		},
	},
	{
		name: "30sectest",
		fixtureFn: func() string {
			return remote.RemoteFixture("1709be323b89b38f20cdb0fd112f0cdc789ebab1cd531e51c11b7fddef9ff238/1779663377.mp4")
		},
	},
}

func TestThumbnail(t *testing.T) {
	for _, tc := range thumbnailTestCases {
		t.Run(tc.name, func(t *testing.T) {
			withNoGSTLeaks(t, func() {
				inputFile, err := os.Open(tc.fixtureFn())
				require.NoError(t, err)
				defer inputFile.Close()
				bs, err := io.ReadAll(inputFile)
				require.NoError(t, err)

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				ctx = log.WithDebugValue(ctx, map[string]map[string]int{"function": {"Thumbnail": 9}})
				g, ctx := errgroup.WithContext(ctx)

				for i := 0; i < streamplaceTestCount; i++ {
					// g.Go(func() error {
					// 	thumbnail := bytes.Buffer{}
					// 	err := Thumbnail(ctx, bytes.NewReader(bs), &thumbnail, "png")
					// 	if err != nil {
					// 		return err
					// 	}
					// 	if thumbnail.Len() == 0 {
					// 		return fmt.Errorf("thumbnail buffer is empty")
					// 	}
					// 	// No strict length checks for muxl variant, but keep sample-segment's as before.
					// 	if tc.name == "sample-segment" {
					// 		require.Equal(t, 1418910, thumbnail.Len())
					// 	} else {
					// 		require.Greater(t, thumbnail.Len(), 50000)
					// 	}
					// 	return nil
					// })
					g.Go(func() error {
						start := time.Now()
						thumbnail := bytes.Buffer{}
						err := Thumbnail(ctx, bytes.NewReader(bs), &thumbnail, "jpeg")
						if err != nil {
							return err
						}
						if thumbnail.Len() == 0 {
							return fmt.Errorf("thumbnail buffer is empty")
						}
						// For jpeg, apply broad range checking for muxl, strict for sample-segment
						require.Greater(t, thumbnail.Len(), 100)
						require.WithinDuration(t, start, time.Now(), 10*time.Second)
						return nil
					})
				}

				err = g.Wait()
				require.NoError(t, err)
			})
		})
	}
}

// TestThumbnailFromSegment exercises the VOD thumbnail decode path
// (thumbnailFromMP4) by hammering the decodebin pipeline on each fixture
// under the leak checker. The fragmented fmp4 fixtures are fed straight to
// decodebin — no muxl flatten — matching what ThumbnailFromSegment now does.
func TestThumbnailFromSegment(t *testing.T) {
	for _, tc := range thumbnailTestCases {
		t.Run(tc.name, func(t *testing.T) {
			withNoGSTLeaks(t, func() {
				bs, err := os.ReadFile(tc.fixtureFn())
				require.NoError(t, err)

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				g, ctx := errgroup.WithContext(ctx)
				for i := 0; i < streamplaceTestCount; i++ {
					g.Go(func() error {
						var thumbnail bytes.Buffer
						if err := thumbnailFromMP4(ctx, bs, &thumbnail, "jpeg"); err != nil {
							return err
						}
						if thumbnail.Len() == 0 {
							return fmt.Errorf("thumbnail buffer is empty")
						}
						return nil
					})
				}
				require.NoError(t, g.Wait())
			})
		})
	}
}

// This segment once caused a segfault in gst-libav.
// It doesn't gotta work but it does gotta not crash.
func TestThumbnailKryptonite(t *testing.T) {
	withNoGSTLeaks(t, func() {
		inputFile, err := os.Open(remote.RemoteFixture("46c876d5e6c4124275b8856431833adaad31cb5246caca8ded9dc4d37de400a4/kryptonite-screenshot.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		thumbnail := bytes.Buffer{}
		err = Thumbnail(context.Background(), bytes.NewReader(bs), &thumbnail, "png")
		require.NoError(t, err)
		require.Equal(t, 561486, thumbnail.Len())
	})
}

// This segment once caused the jpeg encoder to stall.
// So now we have snapshot=false.
func TestThumbnailStall(t *testing.T) {
	withNoGSTLeaks(t, func() {
		inputFile, err := os.Open(remote.RemoteFixture("aef704b702d24de7cf2ae453f4def763f3b39f4f353c8a1602f59cb995aafb53/broken-thumbnail.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)
		thumbnail := bytes.Buffer{}
		err = Thumbnail(context.Background(), bytes.NewReader(bs), &thumbnail, "jpeg")
		require.NoError(t, err)
		// This is inconsistent. Which is concerning.
		// Testing gave ~22000 bytes
		require.Greater(t, thumbnail.Len(), 20000)
		require.Less(t, thumbnail.Len(), 25000)
	})
}
