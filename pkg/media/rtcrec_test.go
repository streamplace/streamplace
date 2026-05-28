package media

import (
	"context"
	"os"
	"testing"

	"github.com/cenkalti/backoff/v5"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/rtcrec"
	"stream.place/streamplace/pkg/statedb"
)

var RTCRecTestCases = []struct {
	name                string
	fatalErrors         bool
	fixture             string
	expectedSegmentsMin int
	expectedSegmentsMax int
}{
	{
		name:                "IntermittentTracks",
		fatalErrors:         false,
		fixture:             "/home/iameli/code/streamplace/out.cbor",
		expectedSegmentsMin: 10,
		expectedSegmentsMax: 15,
	},
}

func TestRTCRecording(t *testing.T) {

	previous := FatalSegmentationErrors
	defer func() {
		FatalSegmentationErrors = previous
	}()
	// ctx := context.Background()
	// mm, ms := getStaticTestMediaManager(t)
	for _, testCase := range RTCRecTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			withNoGSTLeaks(t, func() {
				ctx := context.Background()
				dir, err := os.MkdirTemp("", "rtcrec-test-*")
				require.NoError(t, err)
				defer os.RemoveAll(dir)
				cli := &config.CLI{
					WideOpen: true,
					DataDir:  dir,
					DBURL:    "sqlite://:memory:",
				}
				// fs := cli.NewFlagSet("rtcrec-test")
				// err = cli.Parse(fs, []string{
				// 	"--data-dir", dir,
				// 	"-wide-open=true",
				// 	"--segment-debug-dir", "/home/iameli/testvids/nekomimi.pet",
				// })
				//
				mod, err := model.MakeDB(":memory:")
				require.NoError(t, err)
				ldb, err := localdb.MakeDB(":memory:")
				require.NoError(t, err)
				state, err := statedb.MakeDB(ctx, cli, nil, mod)
				require.NoError(t, err)
				serverHandle, err := atproto.MakeServerRepo(ctx, cli, state)
				require.NoError(t, err)
				defer serverHandle.Close()
				mm, err := MakeMediaManager(context.Background(), cli, nil, mod, nil, nil, ldb)
				require.NoError(t, err)
				priv, pub, err := spkey.GenerateStreamKey()
				require.NoError(t, err)
				signer, err := spkey.KeyToSigner(priv)
				require.NoError(t, err)
				mediaSigner, err := MakeMediaSigner(ctx, cli, pub.DIDKey(), signer, mod)
				require.NoError(t, err)

				segsub := mm.NewSegment()
				segCount := 0
				go func() {
					for range segsub {
						segCount++
					}
				}()

				cur := goleak.IgnoreCurrent()
				defer goleak.VerifyNone(t, cur)

				FatalSegmentationErrors = testCase.fatalErrors
				fd, err := os.Open(testCase.fixture)
				require.NoError(t, err)
				defer fd.Close()
				pc, err := rtcrec.NewReplayPeerConnection(ctx, fd)
				require.NoError(t, err)
				done := make(chan error)
				_, err = mm.WebRTCIngest(ctx, &webrtc.SessionDescription{SDP: "placeholder"}, mediaSigner, pc, done)
				require.NoError(t, err)
				// fmt.Println(answer.SDP)
				<-done

				// the segment getting ingested is ever so slightly after the done, which doesn't matter except in tests, just do a backoff for checking
				ticker := backoff.NewTicker(backoff.NewExponentialBackOff())
				defer ticker.Stop()
				for i := 0; i < 10; i++ {
					if segCount >= testCase.expectedSegmentsMin {
						break
					}
					if i < 9 {
						<-ticker.C
					}
				}
				require.GreaterOrEqual(t, segCount, testCase.expectedSegmentsMin)
				require.LessOrEqual(t, segCount, testCase.expectedSegmentsMax)
			})
		})
	}

}
