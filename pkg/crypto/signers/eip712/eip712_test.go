package eip712_test

import (
	"strings"
	"testing"
	"time"

	"aquareum.tv/aquareum/pkg/crypto/signers/eip712"
	"aquareum.tv/aquareum/pkg/crypto/signers/eip712/eip712test"
	v0 "aquareum.tv/aquareum/pkg/schema/v0"
	"github.com/stretchr/testify/require"
)

func TestEIP712Map(t *testing.T) {
	msg := eip712.AquareumEIP712Message{
		MsgData:   map[string]string{"foo": "bar"},
		MsgSigner: "0x295481766f43bb048aec5d71f3bf76fdacea78f2",
		MsgTime:   time.Now().UnixMilli(),
	}
	m := msg.Map()
	require.Equal(t, m["signer"], msg.MsgSigner)
}

func TestCreateSigner(t *testing.T) {
	ran := false
	eip712test.WithTestSigner(func(signer *eip712.EIP712Signer) {
		ran = true
	})
	require.True(t, ran)
}

func TestSignIdentity(t *testing.T) {
	eip712test.WithTestSigner(func(signer *eip712.EIP712Signer) {
		identity := v0.Identity{
			Handle: "aquareum.bsky.social",
			DID:    "did:plc:dkh4rwafdcda4ko7lewe43ml",
		}
		_, err := signer.SignMessage(identity)
		require.NoError(t, err)
	})
}

var testCase = `{
  "primaryType": "Identity",
  "domain": {
    "name": "Aquareum",
    "version": "0.0.1",
    "chainId": null,
    "verifyingContract": "",
    "salt": ""
  },
  "message": {
    "signer": "0x9153c114d47aceb691b77b02122cb378074e45c8",
    "time": 1732561417949,
    "data": {
      "handle": "aquareum.bsky.social",
      "did": "did:plc:dkh4rwafdcda4ko7lewe43ml"
    }
  },
  "signature": "0xc75ca7da2d110c562eaa4a906aae7a246b2d96a867b74baf3ac0d9127f260dfb17b9aba7d20562c10c771b658c17a4be2dfc427c3e729a07853e35753a8a70f61b"
}`

func TestVerifyIdentity(t *testing.T) {
	eip712test.WithTestSigner(func(signer *eip712.EIP712Signer) {
		signed, err := signer.Verify([]byte(testCase))
		require.NoError(t, err)
		require.Equal(t, signed.Signer(), "0x9153c114d47aceb691b77b02122cb378074e45c8")
		require.Equal(t, signed.Time(), int64(1732561417949))
		identity, ok := signed.Data().(*v0.Identity)
		require.True(t, ok)
		require.Equal(t, identity.Handle, "aquareum.bsky.social")
		require.Equal(t, identity.DID, "did:plc:dkh4rwafdcda4ko7lewe43ml")
	})
}

func TestFailingGoLive(t *testing.T) {
	eip712test.WithTestSigner(func(signer *eip712.EIP712Signer) {
		failingTestCase := strings.Replace(testCase, "aquareum.bsky.social", "evilhandle.evil", 1)
		_, err := signer.Verify([]byte(failingTestCase))
		require.Error(t, err)
	})
}
