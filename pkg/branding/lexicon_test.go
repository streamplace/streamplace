package branding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestRecordLexiconUpToDate fails when vocab.go changed without the record
// lexicon. Regenerate with
//
//	UPDATE_LEXICON=1 go test ./pkg/branding -run TestRecordLexiconUpToDate
//
// then `make lexicons` for the Go/JS bindings.
func TestRecordLexiconUpToDate(t *testing.T) {
	want, err := RecordLexicon()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join("..", "..", LexiconPath)
	if os.Getenv("UPDATE_LEXICON") != "" {
		if err := os.WriteFile(p, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Compared as JSON values: the checked-in file is prettier-formatted.
	var gotV, wantV any
	if err := json.Unmarshal(got, &gotV); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantV); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotV, wantV) {
		t.Fatalf("%s is stale; run UPDATE_LEXICON=1 go test ./pkg/branding -run TestRecordLexiconUpToDate, then make lexicons", LexiconPath)
	}
}
