package media

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/test/remote"
)

type ConvergeCase struct {
	Name        string
	File        string
	Success     bool
	DoH264Parse bool
}

var ConvergeCases = []ConvergeCase{
	{
		Name:        "Good",
		File:        remote.RemoteFixture("dbce5682132f9f1a8d92e1dcd66da99e4ae6eefd7429e4b168ed05d721a80379/2025-11-14T21-10-51-750Z-attempt-000.mp4"),
		Success:     true,
		DoH264Parse: true,
	},
	{
		Name:        "Evil",
		File:        remote.RemoteFixture("d81395168f8b2f3361d8e6d3443eeb678285a1973dc0b31e966cb81f5916db48/2025-11-14T21-10-57-754Z-attempt-000.mp4"),
		Success:     true,
		DoH264Parse: true,
	},
	{
		Name:        "Stuck",
		File:        remote.RemoteFixture("77e32825eaa9dfb8f6c7bbe3cb21213ffa01c1dc0d041f8e3e9cc4d107c95f16/2025-11-17T01-08-56-070Z-converge-segment-did-key-zQ3shX7nQpEqXEp3XFSPkS7mtUjQ3S1MNvxrEP2HeiwyPqmoz.mp4"),
		Success:     true,
		DoH264Parse: false,
	},
	{
		Name:        "CouldNotMultiplex",
		File:        remote.RemoteFixture("c12696bde28c5ab3ffd7040fd7da2ca6d871bb73b40fc2310dece0f6a082ee8a/2025-11-20T18-02-37-957Z-attempt-000.mp4"),
		Success:     true,
		DoH264Parse: false,
	},
	// {
	// 	Name: "CrashedPipeline",
	// 	File: "/Users/iameli/testvids/stuck-converge/2025-11-17T01-08-56-070Z-converge-segment-did-key-zQ3shX7nQpEqXEp3XFSPkS7mtUjQ3S1MNvxrEP2HeiwyPqmoz.mp4",
	// },
}

func TestConvergeSegment(t *testing.T) {
	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatDemuxBin": 9, "ConcatBin": 9}})
	for _, tc := range ConvergeCases {
		tc := tc // capture for parallel tests if ever used
		t.Run(tc.Name, func(t *testing.T) {
			withNoGSTLeaks(t, func() {
				t.Log("--------------")
				t.Logf("test case: %s", tc.File)
				bs, err := os.ReadFile(tc.File)
				require.NoError(t, err)
				bs, err = ConvergeSegment(ctx, &config.CLI{}, bs, 0, "test-streamer", tc.DoH264Parse)
				if tc.Success {
					require.NoError(t, err)
					require.Greater(t, len(bs), 0)
				} else {
					require.Error(t, err)
				}
			})
		})
	}
}
