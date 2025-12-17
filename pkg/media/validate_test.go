package media

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	ct "stream.place/streamplace/pkg/config/configtesting"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

func TestSignMP4(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx := context.Background()

		streamerPriv, streamerPub, err := spkey.GenerateStreamKey()
		require.NoError(t, err)
		streamerSigner, err := spkey.KeyToSigner(streamerPriv)
		require.NoError(t, err)

		publisherPriv, _, err := spkey.GenerateStreamKey()
		require.NoError(t, err)
		publisherSigner, err := spkey.KeyToSigner(publisherPriv)
		require.NoError(t, err)

		cli := ct.CLI(t, &config.CLI{
			TAURL:    "http://timestamp.digicert.com",
			WideOpen: true,
		})

		ms, err := MakeMediaSigner(ctx, cli, streamerPub.DIDKey(), streamerSigner, publisherSigner, nil)
		require.NoError(t, err)

		inputBs, err := os.ReadFile(getFixture("5sec.mp4"))
		require.NoError(t, err)

		streamerSignedBs, err := ms.SignMP4(ctx, bytes.NewReader(inputBs), 0)
		require.NoError(t, err)
		require.NotEmpty(t, streamerSignedBs)
		require.Greater(t, len(streamerSignedBs), len(inputBs))

		doubleSigned, err := ms.SignMP4Publisher(ctx, bytes.NewReader(streamerSignedBs))
		require.NoError(t, err)
		require.NotEmpty(t, doubleSigned)
		// XXX: why doesn't this pass?
		// require.Greater(t, len(doubleSigned), len(streamerSignedBs))
	})
}

func TestSignAndValidateMP4(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx := context.Background()

		streamerPriv, streamerPub, err := spkey.GenerateStreamKey()
		require.NoError(t, err)
		streamerSigner, err := spkey.KeyToSigner(streamerPriv)
		require.NoError(t, err)

		publisherPriv, publisherPub, err := spkey.GenerateStreamKey()
		require.NoError(t, err)
		publisherSigner, err := spkey.KeyToSigner(publisherPriv)
		require.NoError(t, err)

		cli := ct.CLI(t, &config.CLI{
			TAURL:    "http://timestamp.digicert.com",
			WideOpen: true,
		})

		ms, err := MakeMediaSigner(ctx, cli, streamerPub.DIDKey(), streamerSigner, publisherSigner, nil)
		require.NoError(t, err)

		inputBs, err := os.ReadFile(getFixture("5sec.mp4"))
		require.NoError(t, err)

		streamerSignedBs, err := ms.SignMP4(ctx, bytes.NewReader(inputBs), 0)
		require.NoError(t, err)

		doubleSigned, err := ms.SignMP4Publisher(ctx, bytes.NewReader(streamerSignedBs))
		require.NoError(t, err)

		validationResult, err := ValidateMP4Media(ctx, doubleSigned)
		require.NoError(t, err)
		require.NotNil(t, validationResult)
		require.Equal(t, streamerPub.DIDKey(), validationResult.Pub.DIDKey())
		require.Equal(t, publisherPub.DIDKey(), validationResult.Publisher.Pub.DIDKey())
	})
}

func TestValidateMP4Media(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx := context.Background()

		inputBs, err := os.ReadFile(getFixture("sample-segment.mp4"))
		require.NoError(t, err)

		validationResult, err := ValidateMP4Media(ctx, inputBs)
		require.NoError(t, err)
		require.NotNil(t, validationResult)
		require.NotNil(t, validationResult.Pub)
		require.NotNil(t, validationResult.Publisher.Pub)
		require.NotNil(t, validationResult.Meta)
	})
}

func TestValidateMP4(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx := context.Background()

		inputBs, err := os.ReadFile(getFixture("sample-segment.mp4"))
		require.NoError(t, err)

		validationResult, err := ValidateMP4Media(ctx, inputBs)
		require.NoError(t, err)

		streamerKeyDID := validationResult.Pub.DIDKey()
		publisherKeyDID := validationResult.Publisher.Pub.DIDKey()
		repoDID := validationResult.Meta.Creator

		mod, err := model.MakeDB(":memory:")
		require.NoError(t, err)

		dir, err := os.MkdirTemp("", "validate-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cli := ct.CLI(t, &config.CLI{
			AllowedStreams: []string{repoDID},
			DataDir:        dir,
		})

		state, err := statedb.MakeDB(ctx, cli, nil, mod)
		require.NoError(t, err)

		_, _, err = state.EnsurePublisherKey(ctx)
		require.NoError(t, err)

		atsync := &atproto.ATProtoSynchronizer{
			CLI:        cli,
			Model:      mod,
			StatefulDB: state,
			Bus:        bus.NewBus(),
		}

		mm, err := MakeMediaManager(ctx, cli, nil, mod, bus.NewBus(), atsync)
		require.NoError(t, err)

		err = mod.UpdateSigningKey(&model.SigningKey{
			DID:       streamerKeyDID,
			RepoDID:   repoDID,
			RKey:      "test-key",
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		err = mod.UpdatePublisherKey(&model.PublisherKey{
			DID:       publisherKeyDID,
			RepoDID:   repoDID,
			RKey:      "test-publisher-key",
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		err = mm.ValidateMP4(ctx, bytes.NewReader(inputBs), false)
		require.NoError(t, err)
	})
}
