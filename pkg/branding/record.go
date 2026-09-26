package branding

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	glex "github.com/streamplace/glex/runtime"

	"stream.place/streamplace/pkg/placestream"
)

// BlobUploader turns an image into a blob ref in the repo the record is
// written to (com.atproto.repo.uploadBlob on a PDS; the node's own blob
// store for its server repo).
type BlobUploader func(ctx context.Context, key string, v Value) (*glex.Blob, error)

// ToRecord builds the brand record for values, uploading each image.
func ToRecord(ctx context.Context, values Values, upload BlobUploader) (*placestream.BrandingBrand, error) {
	m := map[string]any{}
	for _, key := range values.Keys() {
		v := values[key]
		spec, ok := Lookup(key)
		if !ok {
			// The record only has room for keys this build knows.
			continue
		}
		if spec.Kind == KindImage {
			if len(v.Data) == 0 {
				continue
			}
			blob, err := upload(ctx, key, v)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			m[key] = blob
			continue
		}
		text := string(v.Data)
		if text == "" {
			continue
		}
		if _, typed := jsonDefs[key]; typed {
			var parsed any
			if err := json.Unmarshal(v.Data, &parsed); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			m[key] = parsed
			continue
		}
		m[key] = text
	}
	bs, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var rec placestream.BrandingBrand
	if err := json.Unmarshal(bs, &rec); err != nil {
		return nil, fmt.Errorf("brand record: %w", err)
	}
	return &rec, nil
}

// RecordContent is a brand record taken apart: its text values, ready to
// store, and the blobs its images still have to be fetched from.
type RecordContent struct {
	Text     map[string][]byte
	Blobs    map[string]glex.Blob
	Warnings []string
}

// FromRecord takes a brand record apart. Every text value goes through
// Normalize, the same validation the admin API applies, so a record written
// by another client cannot put a value on the page the admin could not;
// invalid values are dropped with a warning rather than failing the brand.
func FromRecord(rec *placestream.BrandingBrand) (*RecordContent, error) {
	bs, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(bs, &raw); err != nil {
		return nil, err
	}
	out := &RecordContent{Text: map[string][]byte{}, Blobs: map[string]glex.Blob{}}
	for key, val := range raw {
		spec, ok := Lookup(key)
		if !ok {
			continue // $type, and keys from a newer vocabulary
		}
		if spec.Kind == KindImage {
			var blob glex.Blob
			if err := json.Unmarshal(val, &blob); err != nil || !blob.Ref.Defined() {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s: not a blob", key))
				continue
			}
			if blob.Size > int64(spec.MaxSize) {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s: larger than %d bytes", key, spec.MaxSize))
				continue
			}
			out.Blobs[key] = blob
			continue
		}
		var text string
		if _, typed := jsonDefs[key]; typed {
			text = string(val)
		} else if err := json.Unmarshal(val, &text); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s: expected a string", key))
			continue
		}
		if len(text) > spec.MaxSize {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s: larger than %d bytes", key, spec.MaxSize))
			continue
		}
		canon, err := Normalize(key, []byte(text))
		if err != nil {
			out.Warnings = append(out.Warnings, err.Error())
			continue
		}
		if len(canon) > 0 {
			out.Text[key] = canon
		}
	}
	sort.Strings(out.Warnings)
	return out, nil
}
