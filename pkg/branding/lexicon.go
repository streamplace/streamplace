package branding

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RecordNSID is the record a brand is published as: one per hostname, keyed
// by it (at://<owner>/place.stream.branding.brand/live.example.com). A
// node's own brand lives in its server repo; a custom domain's in the repo
// of the account that owns the domain.
const RecordNSID = "place.stream.branding.brand"

// LexiconPath is where RecordLexicon is checked in, relative to the repo
// root. TestRecordLexiconUpToDate keeps it in step with Specs.
const LexiconPath = "lexicons/place/stream/branding/brand.json"

// imageAccept is what a brand's image blobs may be.
var imageAccept = []string{"image/svg+xml", "image/png", "image/jpeg", "image/webp", "image/gif", "image/x-icon", "image/vnd.microsoft.icon"}

// jsonDefs are the typed shapes of the JSON-kind keys; a JSON key missing
// here is carried as a string holding the JSON document (appStory, whose
// geometry has fractional numbers the atproto data model cannot hold).
var jsonDefs = map[string]obj{
	"navLinks":    {{"type", "array"}, {"items", obj{{"type", "ref"}, {"ref", "#link"}}}},
	"socialLinks": {{"type", "array"}, {"items", obj{{"type", "ref"}, {"ref", "#link"}}}},
	"legalLinks":  {{"type", "array"}, {"items", obj{{"type", "ref"}, {"ref", "#legalLink"}}}},
	"navCta":      {{"type", "ref"}, {"ref", "#link"}},
	"appColors":   {{"type", "ref"}, {"ref", "#appColors"}},
}

var appColorKeys = []string{"ink", "paper", "iconBackground", "iconForeground", "adaptiveIconBackground", "adaptiveIconForeground", "splashBackground", "splashForeground", "tileBackground", "tileForeground", "tileHairline", "bannerBackground", "bannerForeground"}

// obj is a JSON object that keeps its keys in the order written, so the
// generated lexicon reads like the hand-written ones.
type obj []kv

type kv struct {
	k string
	v any
}

func (o obj) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, e := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(e.k)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(e.v)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// recordProperty is the lexicon property for one key.
func recordProperty(s Spec) obj {
	var p obj
	switch s.Kind {
	case KindImage:
		p = obj{{"type", "blob"}, {"accept", imageAccept}, {"maxSize", s.MaxSize}}
	case KindJSON:
		if def, ok := jsonDefs[s.Key]; ok {
			// A ref carries its own description.
			return def
		}
		p = obj{{"type", "string"}, {"maxLength", s.MaxSize}}
	case KindEnum:
		p = obj{{"type", "string"}, {"knownValues", s.Enum}}
	case KindColor:
		p = obj{{"type", "string"}, {"maxLength", 7}}
	default:
		p = obj{{"type", "string"}, {"maxLength", s.MaxSize}}
	}
	return append(p, kv{"description", s.Doc})
}

// RecordLexicon renders the place.stream.branding.brand lexicon from Specs:
// every key is an optional property of the same name.
func RecordLexicon() ([]byte, error) {
	props := obj{}
	for _, s := range Specs {
		props = append(props, kv{s.Key, recordProperty(s)})
	}
	str := func(max int) obj { return obj{{"type", "string"}, {"maxLength", max}} }
	colorProps := obj{}
	for _, k := range appColorKeys {
		colorProps = append(colorProps, kv{k, str(64)})
	}
	lex := obj{
		{"lexicon", 1},
		{"id", RecordNSID},
		{"defs", obj{
			{"main", obj{
				{"type", "record"},
				{"description", "A brand: how a Streamplace node looks on one hostname, and how apps built for it look. Keyed by the hostname. Generated from pkg/branding/vocab.go; do not edit by hand."},
				{"key", "any"},
				{"record", obj{
					{"type", "object"},
					{"required", []string{}},
					{"properties", props},
				}},
			}},
			{"link", obj{
				{"type", "object"},
				{"required", []string{"label", "url"}},
				{"properties", obj{
					{"label", str(256)},
					{"url", str(2048)},
					{"icon", append(str(256), kv{"description", "A built-in icon name or, in socialLinks, socialIcon1..4."})},
				}},
			}},
			{"legalLink", obj{
				{"type", "object"},
				{"required", []string{"text", "url"}},
				{"properties", obj{
					{"text", str(256)},
					{"url", str(2048)},
				}},
			}},
			{"appColors", obj{
				{"type", "object"},
				{"description", "Icon and splash colors of a built app; any CSS color."},
				{"properties", colorProps},
			}},
		}},
	}
	bs, err := json.Marshal(lex)
	if err != nil {
		return nil, fmt.Errorf("render lexicon: %w", err)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bs, "", "  "); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
