package reposync

import (
	"context"
	"sort"

	"github.com/ipfs/go-cid"
)

// CollectPrefix walks every record under prefix and returns a path -> record CID
// map. The record bytes are not retained; a caller that needs them should use
// [Walker.WalkPrefix] directly.
func (w *Walker) CollectPrefix(ctx context.Context, root cid.Cid, prefix string) (map[string]cid.Cid, error) {
	return w.CollectRanges(ctx, root, []KeyRange{PrefixRange(prefix)})
}

// CollectRanges is [Walker.CollectPrefix] over an arbitrary set of ranges.
func (w *Walker) CollectRanges(ctx context.Context, root cid.Cid, ranges []KeyRange) (map[string]cid.Cid, error) {
	out := map[string]cid.Cid{}
	err := w.WalkRanges(ctx, root, ranges, func(path string, rcid cid.Cid, _ []byte) error {
		out[path] = rcid
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Diff is the change between two collections of records, as paths.
type Diff struct {
	// Created are paths present in the new collection only.
	Created []string
	// Updated are paths present in both but with a different record CID.
	Updated []string
	// Deleted are paths present in the old collection only.
	Deleted []string
}

// Empty reports whether nothing changed.
func (d Diff) Empty() bool {
	return len(d.Created) == 0 && len(d.Updated) == 0 && len(d.Deleted) == 0
}

// DiffCollections compares two [Walker.CollectPrefix] results. Because a walk is
// complete over its range, a path missing from cur really is gone from the repo,
// so Deleted is safe to apply to a local index. Each list is sorted.
func DiffCollections(prev, cur map[string]cid.Cid) Diff {
	var d Diff
	for path, c := range cur {
		old, ok := prev[path]
		switch {
		case !ok:
			d.Created = append(d.Created, path)
		case !old.Equals(c):
			d.Updated = append(d.Updated, path)
		}
	}
	for path := range prev {
		if _, ok := cur[path]; !ok {
			d.Deleted = append(d.Deleted, path)
		}
	}
	sort.Strings(d.Created)
	sort.Strings(d.Updated)
	sort.Strings(d.Deleted)
	return d
}
