package media

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/globalerror"
	"stream.place/streamplace/pkg/replication/boring"
	"stream.place/streamplace/pkg/rtcrec"
)

func TestRTCRecording(t *testing.T) {
	withNoGSTLeaks(t, func() {
		globalerror.GlobalErrors = []error{}
		ctx := context.Background()
		dir, err := os.MkdirTemp("", "rtcrec-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)
		cli := &config.CLI{}
		fs := cli.NewFlagSet("rtcrec-test")
		err = cli.Parse(fs, []string{"--data-dir", dir, "-wide-open=true"})
		require.NoError(t, err)
		mm, err := MakeMediaManager(context.Background(), cli, nil, &boring.BoringReplicator{}, nil, nil, nil)
		require.NoError(t, err)
		priv, pub, err := spkey.GenerateStreamKey()
		require.NoError(t, err)
		signer, err := spkey.KeyToSigner(priv)
		require.NoError(t, err)
		mediaSigner, err := MakeMediaSigner(ctx, cli, pub.DIDKey(), signer)
		require.NoError(t, err)
		// ctx := context.Background()
		// mm, ms := getStaticTestMediaManager(t)
		fd, err := os.Open(getFixture("intermittent-tracks.cbor"))
		require.NoError(t, err)
		defer fd.Close()
		pc, err := rtcrec.NewReplayPeerConnection(ctx, fd)
		require.NoError(t, err)
		done := make(chan struct{})
		answer, err := mm.WebRTCIngest(ctx, &webrtc.SessionDescription{SDP: "placeholder"}, mediaSigner, pc, done)
		require.NoError(t, err)
		fmt.Println(answer.SDP)
		<-done
		for _, err := range globalerror.GlobalErrors {
			fmt.Printf("got error, non-fatal for now: %v\n", err)
		}
	})
}
