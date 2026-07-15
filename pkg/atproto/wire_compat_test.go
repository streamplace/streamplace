package atproto

import (
	"bytes"
	"testing"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/comatproto"
)

// TestCommitEventIndigoCompat is the regression test for the self-indexing
// outage ("reading repoCommit event: expected cbor array"): a
// subscribeRepos #commit event marshaled with our glex-generated type must be
// decodable by indigo's cbor-gen type, which is what every firehose consumer
// (including our own relay indexer) uses. The killer detail is the required
// `blobs` array: the emitters (CreateServerCommitEvent,
// statedb.CreateCommitEvent) never set it, and a nil slice used to encode as
// CBOR null, which cbor-gen rejects.
func TestCommitEventIndigoCompat(t *testing.T) {
	c, err := cid.Parse("bafyreie5737gdxlw5i64vzichcalba3z2v5n6icifvx5xytvske7mr3hpm")
	require.NoError(t, err)

	// Mirror CommitServerRepoRecord's construction: Blobs deliberately unset.
	commit := &comatproto.SyncSubscribeRepos_Commit{
		Repo:   "did:web:example.com",
		Blocks: glex.Bytes{0x0a},
		Rev:    "3m3mqpsnz34c5",
		Commit: glex.Link(c),
		Time:   time.Now().UTC().Format(time.RFC3339),
		Ops: []comatproto.SyncSubscribeRepos_RepoOp{{
			Action: "create",
			Path:   "place.stream.livestream/3m3mqpsnz34c5",
			Cid:    glex.Link(c),
		}},
		Seq:    42,
		TooBig: false,
	}

	var buf bytes.Buffer
	require.NoError(t, commit.MarshalCBOR(&buf))

	var decoded indigoatproto.SyncSubscribeRepos_Commit
	require.NoError(t, decoded.UnmarshalCBOR(bytes.NewReader(buf.Bytes())),
		"indigo (cbor-gen) must be able to decode our emitted commit event")
	require.Equal(t, "did:web:example.com", decoded.Repo)
	require.Equal(t, int64(42), decoded.Seq)
	require.Len(t, decoded.Ops, 1)
	require.Equal(t, "create", decoded.Ops[0].Action)
	require.Equal(t, "place.stream.livestream/3m3mqpsnz34c5", decoded.Ops[0].Path)

	// On the wire, blobs must be an empty array (0x80), never null (0xf6):
	// cbor-gen decodes a zero-length array back to a nil slice, so check the
	// bytes rather than the decoded value.
	require.Contains(t, buf.String(), "\x65blobs\x80", "blobs must be encoded as an empty array")
	require.NotContains(t, buf.String(), "\x65blobs\xf6", "blobs must not be encoded as null")
}
