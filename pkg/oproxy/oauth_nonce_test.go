package oproxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateValidNonces(t *testing.T) {
	now := time.Unix(1747710294, 0)
	nonces := generateValidNoncesUnhashed("test", now)
	require.Equal(t, []string{
		"test-1747710290000000000",
		"test-1747710280000000000",
		"test-1747710270000000000",
	}, nonces)

	nonces = generateValidNonces("test", now)
	require.Equal(t, []string{
		"90ea642bb90fc42a",
		"c322613dcfa16b24",
		"ba4843b9cac87417",
	}, nonces)
}
