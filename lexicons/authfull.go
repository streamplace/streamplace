package lexicons

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/bluesky-social/indigo/atproto/lexicon"
)

// AuthFullID is the OAuth permission set granting every place.stream record
// collection. Its lexicon, place/stream/authFull.json, is generated from the
// record lexicons by `make lexicons` (gen_authfull.go) and committed like any
// other source lexicon, so the embedded bundle, codegen, and release
// publication all see the same document.
const AuthFullID = "place.stream.authFull"

// PlaceStream parses every place.stream lexicon in bundle: the schemas a
// place.stream lexicon authority publishes. Imported lexicons (app.bsky,
// com.atproto, ...) are skipped. Order follows the bundle's directory walk.
func PlaceStream(bundle fs.FS) ([]*lexicon.SchemaFile, error) {
	lexs := []*lexicon.SchemaFile{}
	err := fs.WalkDir(bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		bs, err := fs.ReadFile(bundle, path)
		if err != nil {
			return err
		}
		lex := &lexicon.SchemaFile{}
		if err := json.Unmarshal(bs, lex); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if !strings.HasPrefix(lex.ID, "place.stream.") {
			return nil
		}
		lexs = append(lexs, lex)
		return nil
	})
	return lexs, err
}

// AuthFull builds place.stream.authFull from lexs: one repo permission over
// every record collection among them, in NSID order so the output is stable.
// Non-record lexicons, including an already generated authFull, contribute
// nothing.
func AuthFull(lexs []*lexicon.SchemaFile) *lexicon.SchemaFile {
	collections := []string{}
	for _, lex := range lexs {
		if _, ok := lex.Defs["main"].Inner.(lexicon.SchemaRecord); ok {
			collections = append(collections, lex.ID)
		}
	}
	slices.Sort(collections)
	title := "Full Streamplace Access"
	detail := "Full access to all Streamplace features and data."
	return &lexicon.SchemaFile{
		Lexicon: 1,
		ID:      AuthFullID,
		Defs: map[string]lexicon.SchemaDef{
			"main": {Inner: lexicon.SchemaPermissionSet{
				Type:   "permission-set",
				Title:  &title,
				Detail: &detail,
				Permissions: []lexicon.SchemaPermission{{
					Type:       "permission",
					Resource:   "repo",
					Collection: collections,
				}},
			}},
		},
	}
}
