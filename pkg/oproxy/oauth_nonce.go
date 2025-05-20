package oproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const EpochLength int64 = int64(time.Second * 10)
const ValidEpochs = 3
const NonceLength = 16

// generate the valid nonces for the current epoch and the previous 2 epochs
// unhashed, for testing and stuff
func generateValidNoncesUnhashed(pad string, now time.Time) []string {
	epochElapsed := now.UnixNano() % EpochLength
	recentEpochStart := now.UnixNano() - epochElapsed
	nonces := make([]string, ValidEpochs)
	for i := 0; i < ValidEpochs; i++ {
		nonces[i] = fmt.Sprintf("%s-%d", pad, recentEpochStart-int64(i)*EpochLength)
	}
	return nonces
}

func generateValidNonces(pad string, now time.Time) []string {
	if pad == "" {
		panic("pad is empty")
	}
	nonces := generateValidNoncesUnhashed(pad, now)
	for i := range nonces {
		nonces[i] = hash(nonces[i])
	}
	return nonces
}

func hash(nonce string) string {
	hash := sha256.Sum256([]byte(nonce))
	str := hex.EncodeToString(hash[:])
	return str[:NonceLength]
}
