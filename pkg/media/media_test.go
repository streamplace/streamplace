package media

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aquareum.tv/aquareum/pkg/config"
	ct "aquareum.tv/aquareum/pkg/config/configtesting"
	"aquareum.tv/aquareum/pkg/crypto/aqpub"
	"aquareum.tv/aquareum/pkg/crypto/signers/eip712/eip712test"
	_ "aquareum.tv/aquareum/pkg/media/mediatesting"
	"aquareum.tv/aquareum/pkg/replication/boring"
	"git.aquareum.tv/aquareum-tv/c2pa-go/pkg/c2pa"
	"github.com/stretchr/testify/require"
)

func getFixture(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "test", "fixtures", name)
}

func getStaticTestMediaManager(t *testing.T) (*MediaManager, *MediaSigner) {
	signer, err := c2pa.MakeStaticSigner(eip712test.KeyBytes)
	require.NoError(t, err)
	pub, err := aqpub.FromHexString("0x16e4f04bc3c9d12fde3d238f60662161d0b87cce")
	if err != nil {
		panic(err)
	}
	cli := ct.CLI(t, &config.CLI{
		TAURL:          "http://timestamp.digicert.com",
		AllowedStreams: []string{pub.String()},
	})
	mm, err := MakeMediaManager(context.Background(), cli, signer, &boring.BoringReplicator{}, nil)
	require.NoError(t, err)
	ms, err := MakeMediaSigner(context.Background(), cli, "test-person", signer, nil)
	return mm, ms
}

// func mp4(t *testing.T) []byte {
// 	f, err := os.Open(getFixture("video.mpegts"))
// 	require.NoError(t, err)
// 	defer f.Close()
// 	buf := bytes.Buffer{}
// 	w := bufio.NewWriter(&buf)
// 	err = MuxToMP4(context.Background(), f, w)
// 	require.NoError(t, err)
// 	return buf.Bytes()
// }

// func TestMuxToMP4(t *testing.T) {
// 	bs := mp4(t)
// 	require.Greater(t, len(bs), 0)
// }

// func TestSignMP4(t *testing.T) {
// 	mp4bs := mp4(t)
// 	r := bytes.NewReader(mp4bs)
// 	_, ms := getStaticTestMediaManager(t)
// 	millis := time.Now().UnixMilli()
// 	bs, err := ms.SignMP4(context.Background(), r, millis)
// 	require.NoError(t, err)
// 	require.Greater(t, len(bs), 0)
// }

// func TestSignMP4WithWallet(t *testing.T) {
// 	eip712test.WithTestSigner(func(signer *eip712.EIP712Signer) {
// 		cli := ct.CLI(t, &config.CLI{
// 			TAURL:          "http://timestamp.digicert.com",
// 			AllowedStreams: []aqpub.Pub{},
// 		})
// 		ms, err := MakeMediaSigner(context.Background(), cli, "test person", signer)
// 		require.NoError(t, err)
// 		mp4bs := mp4(t)
// 		r := bytes.NewReader(mp4bs)
// 		millis := time.Now().UnixMilli()
// 		bs, err := ms.SignMP4(context.Background(), r, millis)
// 		require.NoError(t, err)
// 		require.Greater(t, len(bs), 0)
// 	})
// }

// TODO: Would be good to have this tested with SoftHSM
// func TestSignMP4WithHSM(t *testing.T) {
// 	one := 1
// 	sc, err := crypto11.Configure(&crypto11.Config{
// 		// TokenLabel: "C2PA Signer",
// 		Path:       "/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so",
// 		Pin:        "123456",
// 		SlotNumber: &one,
// 	})
// 	require.NoError(t, err)

// 	allsigners, err := sc.FindAllKeyPairs()
// 	require.NoError(t, err)
// 	signer := allsigners[0]
// 	certBs, err := signers.GenerateES256KCert(signer)
// 	mm := MediaManager{
// 		cli: &config.CLI{
// 			TAURL: "http://timestamp.digicert.com",
// 		},
// 		signer: signer,
// 		cert:   certBs,
// 		user:   "testuser",
// 	}
// 	mp4bs := mp4(t)
// 	r := bytes.NewReader(mp4bs)
// 	f, err := os.CreateTemp("", "*.mp4")
// 	require.NoError(t, err)
// 	ms := time.Now().UnixMilli()
// 	err = mm.SignMP4(context.Background(), r, f, ms)
// 	require.NoError(t, err)
// }

func TestVerifyMP4(t *testing.T) {
	f, err := os.Open(getFixture("sample-segment.mp4"))
	require.NoError(t, err)
	mm, _ := getStaticTestMediaManager(t)
	err = mm.ValidateMP4(context.Background(), f)
	require.NoError(t, err)
}
