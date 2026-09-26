package branding

import "sort"

// Value is one branding key's stored form: canonical text (TextMime) or an
// image's bytes.
type Value struct {
	MimeType string
	Data     []byte
}

// Values is a whole brand, key -> value; the in-memory form every carrier
// (a node's rows, a bundle, a brand directory, a record) converts through.
type Values map[string]Value

// Keys returns the keys in Specs order, unknown keys last.
func (v Values) Keys() []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	order := map[string]int{}
	for i, s := range Specs {
		order[s.Key] = i
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, iok := order[keys[i]]
		oj, jok := order[keys[j]]
		if iok != jok {
			return iok
		}
		if oi != oj {
			return oi < oj
		}
		return keys[i] < keys[j]
	})
	return keys
}
