package placestream

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hyphacoop/go-dasl/drisl"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/appbsky"
)

func TestChatMessageJSONPreservesUnknownFacetFeatures(t *testing.T) {
	const (
		featureType = "place.stream.richtext.facet#emote"
		messageDID  = "did:plc:o6xucog6fghiyrvp7pyqxcs3"
	)

	featureCBOR, err := drisl.Marshal(map[string]any{
		"$type": featureType,
		"name":  "tux",
		"ref": map[string]any{
			"cid": "bafkreife4q4aucn6n3v6elkagmtdzfdr5cdusfrg5ewr6zpcwluy7g7one",
			"uri": "at://did:plc:o6xucog6fghiyrvp7pyqxcs3/place.stream.emote.item/3mg6w5qqzxi25",
		},
	})
	require.NoError(t, err)

	record := ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		CreatedAt:     "2026-09-01T23:36:02.580Z",
		Facets: []RichtextFacet{{
			Features: []RichtextFacet_Features_Elem{{
				Raw: &glex.RawRecord{Type: featureType, Encoding: "cbor", Bytes: featureCBOR},
			}},
			Index: appbsky.RichtextFacet_ByteSlice{ByteStart: 0, ByteEnd: 5},
		}},
		Streamer: messageDID,
		Text:     ":tux:",
	}

	var recordCBOR bytes.Buffer
	require.NoError(t, record.MarshalCBOR(&recordCBOR))

	// Decode the persisted record through the typed envelope path used by the
	// firehose before it is wrapped in the websocket message view.
	var decoded ChatMessage
	require.NoError(t, glex.DecodeCBOR(recordCBOR.Bytes(), &decoded))
	view := ChatDefs_MessageView{
		LexiconTypeID: "place.stream.chat.defs#messageView",
		Record:        &glex.LexiconTypeDecoder{Val: &decoded},
	}

	encoded, err := json.Marshal(view)
	require.NoError(t, err)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))
	recordWire := wire["record"].(map[string]any)
	require.Equal(t, "place.stream.chat.message", recordWire["$type"])
	facetWire := recordWire["facets"].([]any)[0].(map[string]any)
	featureWire := facetWire["features"].([]any)[0].(map[string]any)
	require.Equal(t, featureType, featureWire["$type"])
	require.Equal(t, "tux", featureWire["name"])
	require.Equal(t, map[string]any{
		"cid": "bafkreife4q4aucn6n3v6elkagmtdzfdr5cdusfrg5ewr6zpcwluy7g7one",
		"uri": "at://did:plc:o6xucog6fghiyrvp7pyqxcs3/place.stream.emote.item/3mg6w5qqzxi25",
	}, featureWire["ref"])
}
