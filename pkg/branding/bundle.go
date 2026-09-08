package branding

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"stream.place/streamplace/pkg/statedb"
)

// A bundle is a zip with branding.yaml at its root plus one file per image
// asset. The YAML is the source of truth and is meant to be hand-edited:
//
//	version: 1
//	branding:            # only what is set on the node
//	  siteTitle: Sample
//	  primaryColor: "#11e8b2"
//	  navLinks:          # JSON keys are real YAML structures
//	    - {label: Home, url: https://example.com, icon: home}
//	  mainLogo: mainLogo.svg   # image keys name a file in the zip
//	defaults:            # informational: the app's default for every key
//	  primaryColor: "#6366f1"
//	  ...
//
// Import replaces: keys absent from the bundle are removed from the node
// (or kept, with merge). Every value goes through the same validation as
// the admin API, so a bad color or an oversized image is rejected by name.

const (
	BundleVersion = 1
	yamlName      = "branding.yaml"
	// MaxBundleSize caps an uploaded bundle.
	MaxBundleSize = 8 * 1024 * 1024
)

// Doc is the YAML document.
type Doc struct {
	Version  int                    `yaml:"version"`
	Branding map[string]interface{} `yaml:"branding"`
	Defaults map[string]string      `yaml:"defaults,omitempty"`
}

// Change is one key's transition in an import.
type Change struct {
	Key    string `json:"key"`
	Action string `json:"action"` // added | changed | removed | unchanged
	// Detail is a short human summary: the new text value, or "<mime> <n>B".
	Detail string `json:"detail,omitempty"`
}

// Report is what an import (or dry run) produced.
type Report struct {
	Applied  bool     `json:"applied"`
	Changes  []Change `json:"changes"`
	Warnings []string `json:"warnings,omitempty"`
}

var extByMime = map[string]string{
	"image/svg+xml":            ".svg",
	"image/png":                ".png",
	"image/jpeg":               ".jpg",
	"image/webp":               ".webp",
	"image/x-icon":             ".ico",
	"image/vnd.microsoft.icon": ".ico",
	"image/gif":                ".gif",
}

func fileNameFor(key, mimeType string) string {
	ext, ok := extByMime[strings.ToLower(strings.TrimSpace(mimeType))]
	if !ok {
		exts, _ := mime.ExtensionsByType(mimeType)
		if len(exts) > 0 {
			ext = exts[0]
		} else {
			ext = ".bin"
		}
	}
	return key + ext
}

func mimeFor(name, declared string) string {
	if declared != "" {
		return declared
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".gif":
		return "image/gif"
	}
	return "application/octet-stream"
}

// Export builds a bundle of everything set on broadcasterID.
func Export(ctx context.Context, state *statedb.StatefulDB, broadcasterID string) ([]byte, error) {
	keys, err := state.ListBrandingKeys(broadcasterID)
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)

	doc := Doc{Version: BundleVersion, Branding: map[string]interface{}{}, Defaults: map[string]string{}}
	for _, s := range Specs {
		if s.Kind != KindImage && s.Default != "" {
			doc.Defaults[s.Key] = s.Default
		}
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, key := range keys {
		blob, err := state.GetBrandingBlob(broadcasterID, key)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		spec, known := Lookup(key)
		if !known {
			// Unknown to this build: still round-trip it, as text if it is.
			if blob.MimeType == TextMime {
				doc.Branding[key] = string(blob.Data)
			}
			continue
		}
		if spec.Kind == KindImage {
			if len(blob.Data) == 0 {
				continue
			}
			name := fileNameFor(key, blob.MimeType)
			w, err := zw.Create(name)
			if err != nil {
				return nil, err
			}
			if _, err := w.Write(blob.Data); err != nil {
				return nil, err
			}
			doc.Branding[key] = name
			continue
		}
		v := strings.TrimSpace(string(blob.Data))
		if v == "" {
			continue
		}
		if spec.Kind == KindJSON {
			var parsed interface{}
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				doc.Branding[key] = parsed
				continue
			}
		}
		doc.Branding[key] = v
	}

	yw, err := zw.Create(yamlName)
	if err != nil {
		return nil, err
	}
	header := "# Streamplace branding bundle. Edit branding: and re-import; defaults: is\n# informational. Image keys name files in this zip.\n"
	if _, err := io.WriteString(yw, header); err != nil {
		return nil, err
	}
	enc := yaml.NewEncoder(yw)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// parsed is a bundle after reading and validation, ready to apply.
type parsed struct {
	text   map[string][]byte // key -> canonical value (non-empty)
	images map[string]struct {
		data []byte
		mime string
	}
	warnings []string
}

// Parse reads and validates a bundle without touching the database.
func Parse(zipBytes []byte) (*parsed, error) {
	if len(zipBytes) > MaxBundleSize {
		return nil, fmt.Errorf("bundle is larger than %d bytes", MaxBundleSize)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("not a zip file: %w", err)
	}
	files := map[string]*zip.File{}
	var yamlFile *zip.File
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		if strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") || f.FileInfo().IsDir() {
			continue
		}
		if path.Base(name) == yamlName && yamlFile == nil {
			yamlFile = f
			continue
		}
		files[path.Base(name)] = f
	}
	if yamlFile == nil {
		return nil, fmt.Errorf("bundle has no %s", yamlName)
	}
	read := func(f *zip.File, limit int) ([]byte, error) {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		bs, err := io.ReadAll(io.LimitReader(rc, int64(limit)+1))
		if err != nil {
			return nil, err
		}
		if len(bs) > limit {
			return nil, fmt.Errorf("%s is larger than %d bytes", f.Name, limit)
		}
		return bs, nil
	}
	ybs, err := read(yamlFile, 256*1024)
	if err != nil {
		return nil, err
	}
	var doc Doc
	if err := yaml.Unmarshal(ybs, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", yamlName, err)
	}
	if doc.Version == 0 {
		doc.Version = BundleVersion
	}
	if doc.Version > BundleVersion {
		return nil, fmt.Errorf("bundle version %d is newer than this node understands (%d)", doc.Version, BundleVersion)
	}

	p := &parsed{text: map[string][]byte{}, images: map[string]struct {
		data []byte
		mime string
	}{}}
	keys := make([]string, 0, len(doc.Branding))
	for k := range doc.Branding {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		raw := doc.Branding[key]
		spec, known := Lookup(key)
		if !known {
			p.warnings = append(p.warnings, fmt.Sprintf("%s: unknown key, kept as text", key))
			spec = Spec{Key: key, Kind: KindText, MaxSize: jsonMax}
		}
		if raw == nil {
			continue
		}
		if spec.Kind == KindImage {
			name, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("%s: expected a file name", key)
			}
			f, ok := files[path.Base(name)]
			if !ok {
				return nil, fmt.Errorf("%s: file %q not in bundle", key, name)
			}
			data, err := read(f, spec.MaxSize)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			p.images[key] = struct {
				data []byte
				mime string
			}{data: data, mime: mimeFor(name, "")}
			continue
		}
		var text string
		switch v := raw.(type) {
		case string:
			text = v
		case bool, int, int64, float64:
			text = fmt.Sprint(v)
		default:
			// JSON-valued keys arrive as YAML structures.
			bs, err := json.Marshal(yamlToJSON(v))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			text = string(bs)
		}
		if len(text) > spec.MaxSize {
			return nil, fmt.Errorf("%s is larger than %d bytes", key, spec.MaxSize)
		}
		canon, err := Normalize(key, []byte(text))
		if err != nil {
			return nil, err
		}
		if len(canon) == 0 {
			continue
		}
		p.text[key] = canon
	}
	return p, nil
}

// yamlToJSON converts yaml.v3's map[string]interface{} / []interface{}
// trees (already string-keyed in v3) into something json.Marshal accepts.
func yamlToJSON(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, val := range t {
			out[k] = yamlToJSON(val)
		}
		return out
	case map[interface{}]interface{}:
		out := map[string]interface{}{}
		for k, val := range t {
			out[fmt.Sprint(k)] = yamlToJSON(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, val := range t {
			out[i] = yamlToJSON(val)
		}
		return out
	}
	return v
}

// Import applies a bundle to broadcasterID. With merge, keys absent from
// the bundle are left alone; otherwise they are removed. With dryRun the
// report is computed and nothing is written.
func Import(ctx context.Context, state *statedb.StatefulDB, broadcasterID string, zipBytes []byte, merge, dryRun bool) (*Report, error) {
	p, err := Parse(zipBytes)
	if err != nil {
		return nil, err
	}
	existingKeys, err := state.ListBrandingKeys(broadcasterID)
	if err != nil {
		return nil, err
	}
	existing := map[string]*statedb.BrandingBlob{}
	for _, k := range existingKeys {
		b, err := state.GetBrandingBlob(broadcasterID, k)
		if err == nil && (len(b.Data) > 0) {
			existing[k] = b
		}
	}

	report := &Report{Warnings: p.warnings, Changes: []Change{}}
	type write struct {
		key, mime string
		data      []byte
	}
	var writes []write
	var removes []string

	for key, val := range p.text {
		if cur, ok := existing[key]; ok && sameText(key, cur.Data, val) {
			report.Changes = append(report.Changes, Change{Key: key, Action: "unchanged"})
			continue
		}
		action := "added"
		if _, ok := existing[key]; ok {
			action = "changed"
		}
		detail := string(val)
		if len(detail) > 80 {
			detail = detail[:77] + "..."
		}
		report.Changes = append(report.Changes, Change{Key: key, Action: action, Detail: detail})
		writes = append(writes, write{key: key, mime: TextMime, data: val})
	}
	for key, img := range p.images {
		if cur, ok := existing[key]; ok && bytes.Equal(cur.Data, img.data) {
			report.Changes = append(report.Changes, Change{Key: key, Action: "unchanged"})
			continue
		}
		action := "added"
		if _, ok := existing[key]; ok {
			action = "changed"
		}
		report.Changes = append(report.Changes, Change{Key: key, Action: action, Detail: fmt.Sprintf("%s %dB", img.mime, len(img.data))})
		writes = append(writes, write{key: key, mime: img.mime, data: img.data})
	}
	if !merge {
		for key := range existing {
			if _, inText := p.text[key]; inText {
				continue
			}
			if _, inImg := p.images[key]; inImg {
				continue
			}
			report.Changes = append(report.Changes, Change{Key: key, Action: "removed"})
			removes = append(removes, key)
		}
	}
	sort.Slice(report.Changes, func(i, j int) bool { return report.Changes[i].Key < report.Changes[j].Key })

	if dryRun {
		return report, nil
	}
	for _, w := range writes {
		if err := state.PutBrandingBlob(broadcasterID, w.key, w.mime, w.data, nil, nil); err != nil {
			return nil, fmt.Errorf("write %s: %w", w.key, err)
		}
	}
	for _, key := range removes {
		if err := state.DeleteBrandingBlob(broadcasterID, key); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("remove %s: %w", key, err)
		}
	}
	report.Applied = true
	return report, nil
}

// sameText compares a stored text value with a canonical incoming one,
// treating JSON keys semantically (older rows may hold uncanonicalized JSON).
func sameText(key string, stored, incoming []byte) bool {
	if spec, ok := Lookup(key); ok && spec.Kind == KindJSON {
		if c, err := CanonicalJSON(stored); err == nil {
			return bytes.Equal(c, incoming)
		}
	}
	return bytes.Equal(bytes.TrimSpace(stored), incoming)
}
