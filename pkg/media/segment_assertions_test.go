package media

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/c2patypes"
)

// The context and compact field shape match ManifestBuilder's default output.
const coreSegmentAssertion = `{
	"@context": {
		"dc": "http://purl.org/dc/elements/1.1/",
		"Iptc4xmpExt": "http://iptc.org/std/Iptc4xmpExt/2008-02-29/",
		"photoshop": "http://ns.adobe.com/photoshop/1.0/",
		"xmpRights": "http://ns.adobe.com/xap/1.0/rights/"
	},
	"dc:creator": "did:example:streamer",
	"dc:title": "unpublished livestream",
	"dc:date": "2026-10-03T12:00:00.000Z"
}`

func segmentAssertionFixture(t testing.TB) *c2patypes.Manifest {
	t.Helper()
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(coreSegmentAssertion), &data))
	return &c2patypes.Manifest{Assertions: []c2patypes.ManifestAssertion{{Label: StreamplaceMetadata, Data: data}}}
}

// JSON-LD contexts, value forms and cardinalities retain their meaning.
func TestParseSegmentAssertionsJSONLD(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*c2patypes.Manifest)
		creator string
		title   string
		invalid bool
	}{
		{name: "ProductionContext", creator: "did:example:streamer", title: "unpublished livestream"},
		{name: "SignerFallbackContext", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["@context"] = map[string]any{"dc": "http://purl.org/dc/elements/1.1/"}
		}, creator: "did:example:streamer", title: "unpublished livestream"},
		{name: "LegacyAssertionLabel", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Label = "place.stream.metadata"
		}, creator: "did:example:streamer", title: "unpublished livestream"},
		{name: "CreatorAlias", change: func(m *c2patypes.Manifest) {
			d := m.Assertions[0].Data.(map[string]any)
			d["@context"].(map[string]any)["creator"] = "dc:creator"
			d["creator"] = d["dc:creator"]
			delete(d, "dc:creator")
		}, creator: "did:example:streamer", title: "unpublished livestream"},
		{name: "OverriddenTitleTerm", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["@context"].(map[string]any)["dc:title"] = "http://example.com/title"
		}, invalid: true},
		{name: "WrongDCNamespace", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["@context"].(map[string]any)["dc"] = "http://example.com/"
		}, invalid: true},
		{name: "Graph", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data = map[string]any{"@graph": []any{m.Assertions[0].Data}}
		}, creator: "did:example:streamer", title: "unpublished livestream"},
		{name: "MultipleGraphNodes", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data = map[string]any{"@graph": []any{m.Assertions[0].Data, m.Assertions[0].Data}}
		}, invalid: true},
		{name: "ValueObject", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:title"] = map[string]any{"@value": "live", "@language": "en"}
		}, creator: "did:example:streamer", title: "live"},
		{name: "MultipleCreators", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:creator"] = []any{"first", "second"}
		}, creator: "first", title: "unpublished livestream"},
		{name: "NumericCreator", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:creator"] = float64(7)
		}, invalid: true},
		{name: "NumericAdditionalCreator", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:creator"] = []any{"first", float64(7)}
		}, invalid: true},
		{name: "BooleanTitle", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:title"] = true
		}, invalid: true},
		{name: "MissingCreator", change: func(m *c2patypes.Manifest) {
			delete(m.Assertions[0].Data.(map[string]any), "dc:creator")
		}, invalid: true},
		{name: "NullCreator", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:creator"] = nil
		}, invalid: true},
		{name: "MultipleTitles", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:title"] = []any{"first", "second"}
		}, invalid: true},
		{name: "MultipleDates", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:date"] = []any{"2026-10-03T12:00:00.000Z", "2026-10-03T12:00:01.000Z"}
		}, invalid: true},
		{name: "InvalidDate", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:date"] = "yesterday"
		}, invalid: true},
		{name: "InvalidUTF8", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:title"] = string([]byte{0xff})
		}, creator: "did:example:streamer", title: "�"},
		{name: "EmptyStrings", change: func(m *c2patypes.Manifest) {
			d := m.Assertions[0].Data.(map[string]any)
			d["dc:creator"], d["dc:title"] = "", ""
		}, creator: "", title: ""},
		{name: "CreatorNodeReference", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["dc:creator"] = map[string]any{"@id": "did:example:streamer"}
		}, creator: "", title: "unpublished livestream"},
		{name: "CaseInsensitiveExpandedIRIs", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data = map[string]any{
				"http://purl.org/dc/elements/1.1/CREATOR": []any{map[string]any{"@value": "did:example:streamer"}},
				"http://purl.org/dc/elements/1.1/TITLE":   []any{map[string]any{"@value": "unpublished livestream"}},
				"http://purl.org/dc/elements/1.1/DATE":    []any{map[string]any{"@value": "2026-10-03T12:00:00.000Z"}},
			}
		}, creator: "did:example:streamer", title: "unpublished livestream"},
		{name: "UnknownField", change: func(m *c2patypes.Manifest) {
			m.Assertions[0].Data.(map[string]any)["http://example.com/extra"] = map[string]any{"@value": float64(42)}
		}, creator: "did:example:streamer", title: "unpublished livestream"},
		{name: "NullData", change: func(m *c2patypes.Manifest) { m.Assertions[0].Data = nil }, invalid: true},
		{name: "NonMapData", change: func(m *c2patypes.Manifest) { m.Assertions[0].Data = []any{} }, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := segmentAssertionFixture(t)
			if tt.change != nil {
				tt.change(m)
			}
			meta, err := ParseSegmentAssertions(context.Background(), m)
			if tt.invalid {
				require.Error(t, err)
				require.Nil(t, meta)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.creator, meta.Creator)
			require.Equal(t, tt.title, meta.Title)
			require.Equal(t, aqtime.AQTime("2026-10-03T12:00:00.000Z"), meta.StartTime)
		})
	}
}

func TestParseSegmentAssertionsOptionalMetadata(t *testing.T) {
	m := segmentAssertionFixture(t)
	d := m.Assertions[0].Data.(map[string]any)
	d["dc:rights"] = "copyright streamer"
	d["Iptc4xmpExt:CopyrightYear"] = float64(2026)
	d["photoshop:Credit"] = "camera team"
	d["xmpRights:UsageTerms"] = "all rights reserved"
	d["Iptc4xmpExt:ContentWarning"] = []any{"flashing lights"}
	m.Assertions = append(m.Assertions,
		c2patypes.ManifestAssertion{Label: "place.stream.metadata.configuration", Data: map[string]any{"distributionPolicy": map[string]any{"deleteAfter": float64(300), "allowGenAiTraining": false}}},
		c2patypes.ManifestAssertion{Label: C2PAActionsV2Label, Data: map[string]any{"actions": []any{map[string]any{"action": C2PAPublishedAction}}}},
	)
	meta, err := ParseSegmentAssertions(context.Background(), m)
	require.NoError(t, err)
	require.Equal(t, "did:example:streamer", meta.Creator)
	require.Equal(t, "unpublished livestream", meta.Title)
	require.Equal(t, []string{"flashing lights"}, meta.ContentWarnings)
	require.NotNil(t, meta.ContentRights)
	require.Equal(t, "copyright streamer", *meta.ContentRights.CopyrightNotice)
	require.Equal(t, int64(2026), *meta.ContentRights.CopyrightYear)
	require.Equal(t, "camera team", *meta.ContentRights.CreditLine)
	require.Equal(t, "all rights reserved", *meta.ContentRights.License)
	require.NotNil(t, meta.DistributionPolicy)
	require.Equal(t, int64(300), *meta.DistributionPolicy.DeleteAfterSeconds)
	require.False(t, *meta.DistributionPolicy.AllowGenAiTraining)
	require.True(t, meta.Published)
}

func BenchmarkParseSegmentAssertions(b *testing.B) {
	for _, optional := range []bool{false, true} {
		name := "production-core"
		if optional {
			name = "optional-metadata"
		}
		b.Run(name, func(b *testing.B) {
			m := segmentAssertionFixture(b)
			if optional {
				d := m.Assertions[0].Data.(map[string]any)
				d["dc:rights"] = "copyright streamer"
				d["Iptc4xmpExt:CopyrightYear"] = float64(2026)
				d["photoshop:Credit"] = "camera team"
				d["xmpRights:UsageTerms"] = "all rights reserved"
				d["Iptc4xmpExt:ContentWarning"] = []any{"flashing lights"}
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := ParseSegmentAssertions(context.Background(), m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
