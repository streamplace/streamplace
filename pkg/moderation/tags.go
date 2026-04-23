package moderation

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

//go:embed en.b64
var embeddedWordlistB64 string

// ModerationDir is set by the CLI flag --moderation-dir. When non-empty,
// all .txt files in this directory are loaded as additional profanity wordlists.
var ModerationDir string

var (
	blocklist     map[string]struct{}
	blocklistOnce sync.Once
	blocklistErr  error
)

func normalizeTag(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseWords(data string, into map[string]struct{}) {
	for line := range strings.SplitSeq(data, "\n") {
		word := normalizeTag(line)
		if word != "" {
			into[word] = struct{}{}
		}
	}
}

func loadBlocklist() error {
	decoded, err := base64.StdEncoding.DecodeString(embeddedWordlistB64)
	if err != nil {
		return fmt.Errorf("decoding embedded wordlist: %w", err)
	}

	blocklist = make(map[string]struct{})
	parseWords(string(decoded), blocklist)

	if ModerationDir != "" {
		entries, err := os.ReadDir(ModerationDir)
		if err != nil {
			return fmt.Errorf("reading moderation directory %q: %w", ModerationDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".txt" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(ModerationDir, entry.Name()))
			if err != nil {
				return fmt.Errorf("reading wordlist %q: %w", entry.Name(), err)
			}
			parseWords(string(data), blocklist)
		}
	}
	return nil
}

func getBlocklist() (map[string]struct{}, error) {
	blocklistOnce.Do(func() {
		blocklistErr = loadBlocklist()
	})
	return blocklist, blocklistErr
}

// FilterTags returns only the tags that don't match a known profanity term.
func FilterTags(tags []string) []string {
	if len(tags) == 0 {
		return tags
	}
	bl, err := getBlocklist()
	if err != nil || len(bl) == 0 {
		return tags
	}
	out := tags[:0:0]
	for _, tag := range tags {
		if _, blocked := bl[normalizeTag(tag)]; !blocked {
			out = append(out, tag)
		}
	}
	return out
}
