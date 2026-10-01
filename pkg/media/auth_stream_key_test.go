package media

import (
	"context"
	"fmt"
	"testing"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/model"
)

type captionAuthModel struct {
	model.Model
	key    *model.SigningKey
	keyErr error
	labels []*comatproto.LabelDefs_Label
}

func (m *captionAuthModel) GetSigningKey(_ context.Context, did, repo string) (*model.SigningKey, error) {
	if m.key != nil && (m.key.DID != did || m.key.RepoDID != repo) {
		return nil, nil
	}
	return m.key, m.keyErr
}
func (m *captionAuthModel) GetActiveLabels(string) ([]*comatproto.LabelDefs_Label, error) {
	return m.labels, nil
}

func TestCaptionStreamKeyAuthorization(t *testing.T) {
	priv, pub, err := spkey.GenerateStreamKey()
	require.NoError(t, err)
	key := "z" + base58.Encode(priv.Bytes())
	did := pub.DIDKey()
	registered := &model.SigningKey{DID: did, RepoDID: did}
	for _, tc := range []struct {
		name string
		cli  config.CLI
		mod  captionAuthModel
		key  string
		err  string
	}{
		{name: "registered", cli: config.CLI{WideOpen: true}, mod: captionAuthModel{key: registered}, key: key},
		{name: "unregistered", cli: config.CLI{WideOpen: true}, key: key, err: "signing key not found"},
		{name: "revoked", cli: config.CLI{WideOpen: true}, mod: captionAuthModel{keyErr: fmt.Errorf("signing key revoked")}, key: key, err: "signing key revoked"},
		{name: "banned", cli: config.CLI{WideOpen: true}, mod: captionAuthModel{key: registered, labels: []*comatproto.LabelDefs_Label{{Uri: did, Val: atproto.LabelDMCAViolation}}}, key: key, err: "user is banned"},
		{name: "disallowed", cli: config.CLI{AllowedStreams: []string{"did:plc:someoneelse"}}, mod: captionAuthModel{key: registered}, key: key, err: "user is not allowed to stream"},
		{name: "malformed short key", cli: config.CLI{WideOpen: true}, key: "z2", err: "invalid authorization key"},
		{name: "malformed base58", cli: config.CLI{WideOpen: true}, key: "z0", err: "invalid authorization key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, signer, err := AuthenticateStreamKey(context.Background(), &tc.cli, &tc.mod, nil, tc.key, true)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				require.Empty(t, got)
				require.Nil(t, signer)
				return
			}
			require.NoError(t, err)
			require.Equal(t, did, got)
			parsed, err := spkey.KeyToSigner(priv)
			require.NoError(t, err)
			require.Equal(t, parsed.Public(), signer.Public())
		})
	}
	// WHIP retains its legacy unregistered did:key mode; caption pushes do not.
	got, _, err := AuthenticateStreamKey(context.Background(), &config.CLI{WideOpen: true}, &captionAuthModel{}, nil, key, false)
	require.NoError(t, err)
	require.Equal(t, did, got)
}
