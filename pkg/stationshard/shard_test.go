package stationshard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStableDIDSlots(t *testing.T) {
	require.Equal(t, uint16(0xa3), Slot("did:plc:alice"))
	require.Equal(t, uint16(0x53), Slot("did:plc:bob"))
	require.Equal(t, uint16(0xfb), Slot("did:web:station.example"))
	// Even collocated identities must not be able to reach each other's keys.
	require.NotEqual(t, dbKey("a", "b\x00c"), dbKey("a\x00b", "c"))
}
