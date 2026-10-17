package branding

import (
	"strings"

	"stream.place/streamplace/pkg/statedb"
)

// Text reads a text-valued branding key for the node whose broadcaster host
// is host, under the did:web key the admin UI writes and, failing that, the
// bare host older nodes used. Unset or unreadable keys read as "".
func Text(state *statedb.StatefulDB, host, key string) string {
	if state == nil || host == "" {
		return ""
	}
	for _, id := range []string{"did:web:" + host, host} {
		blob, err := state.GetBrandingBlob(id, key)
		if err != nil || blob == nil {
			continue
		}
		if blob.MimeType != TextMime {
			return ""
		}
		return strings.TrimSpace(string(blob.Data))
	}
	return ""
}
