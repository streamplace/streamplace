package spaces

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bluesky-social/indigo/xrpc"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/placestream"
)

const (
	PersonalSpaceType = "place.stream.personal"
	PersonalSpaceKey  = "self"
	TargetCollection  = "place.stream.multistream.target"
)

// Client is the XRPC surface used by personal spaces.
type Client interface {
	Do(ctx context.Context, kind string, inpenc, method string, params map[string]any, bodyobj any, out any) error
}

func PersonalSpaceURI(did string) string {
	return "at://" + did + "/space/" + PersonalSpaceType + "/" + PersonalSpaceKey
}

// Granted reports whether an OAuth scope, as a PDS expands it, covers the personal space.
func Granted(scope string) bool {
	return strings.Contains(scope, "space:"+PersonalSpaceType)
}

type recordAddress struct {
	Space      string `json:"space"`
	Repo       string `json:"repo"`
	Collection string `json:"collection"`
	Rkey       string `json:"rkey"`
}

func spaceError(err error, name string) bool {
	var rpcErr *xrpc.XRPCError
	return errors.As(err, &rpcErr) && rpcErr.ErrStr == name
}

// PutRecord upserts a record, creating the personal space on first use.
func PutRecord(ctx context.Context, c Client, did, collection, rkey string, record any) (uri, cid string, err error) {
	body := struct {
		recordAddress
		Record any `json:"record"`
	}{recordAddress{PersonalSpaceURI(did), did, collection, rkey}, record}
	var out struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	put := func() error {
		return c.Do(ctx, "POST", "application/json", "com.atproto.space.putRecord", nil, &body, &out)
	}
	if err = put(); spaceError(err, "SpaceNotFound") {
		create := struct {
			SpaceType string         `json:"spaceType"`
			Skey      string         `json:"skey"`
			Policy    map[string]any `json:"policy"`
		}{PersonalSpaceType, PersonalSpaceKey, map[string]any{"$type": "com.atproto.personalspace.policy"}}
		err = c.Do(ctx, "POST", "application/json", "com.atproto.space.createSpace", nil, &create, nil)
		if err == nil || spaceError(err, "SpaceAlreadyExists") {
			err = put()
		}
	}
	if err != nil {
		return "", "", fmt.Errorf("put personal space record: %w", err)
	}
	return out.URI, out.CID, nil
}

func DeleteRecord(ctx context.Context, c Client, did, collection, rkey string) error {
	body := recordAddress{PersonalSpaceURI(did), did, collection, rkey}
	if err := c.Do(ctx, "POST", "application/json", "com.atproto.space.deleteRecord", nil, &body, nil); err != nil {
		return fmt.Errorf("delete personal space record: %w", err)
	}
	return nil
}

// A space that was never created, or never written to, has no records.
func noRepo(err error) bool {
	return spaceError(err, "SpaceNotFound") || spaceError(err, "RepoNotFound")
}

// Record is one record of a personal space.
type Record struct {
	Rkey  string          `json:"rkey"`
	CID   string          `json:"cid"`
	Value json.RawMessage `json:"value"`
}

// ListRecords returns every record of collection in did's personal space.
func ListRecords(ctx context.Context, c Client, did, collection string) ([]Record, error) {
	params := map[string]any{"space": PersonalSpaceURI(did), "repo": did, "collection": collection, "limit": 1000}
	var records []Record
	for {
		var out struct {
			Cursor  string   `json:"cursor"`
			Records []Record `json:"records"`
		}
		err := c.Do(ctx, "GET", "", "com.atproto.space.listRecords", params, nil, &out)
		if noRepo(err) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("list personal space records: %w", err)
		}
		records = append(records, out.Records...)
		if out.Cursor == "" || len(out.Records) == 0 {
			return records, nil
		}
		params["cursor"] = out.Cursor
	}
}

// LatestRev returns the revision of did's repo in their personal space, which
// changes with every write; it is empty before the first one.
func LatestRev(ctx context.Context, c Client, did string) (string, error) {
	var out struct {
		Commit struct {
			Rev string `json:"rev"`
		} `json:"commit"`
	}
	params := map[string]any{"space": PersonalSpaceURI(did), "repo": did}
	err := c.Do(ctx, "GET", "", "com.atproto.space.getLatestCommit", params, nil, &out)
	if err != nil && !noRepo(err) {
		return "", fmt.Errorf("get personal space commit: %w", err)
	}
	return out.Commit.Rev, nil
}

// TargetURI is the URI the API gives a multistream target, whichever store holds it.
func TargetURI(did, rkey string) string {
	return "at://" + did + "/" + TargetCollection + "/" + rkey
}

// ListMultistreamTargets returns did's multistream targets from their personal
// space, in rkey order.
func ListMultistreamTargets(ctx context.Context, c Client, did string) ([]placestream.MultistreamDefs_TargetView, error) {
	records, err := ListRecords(ctx, c, did, TargetCollection)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(records, func(a, b Record) int { return strings.Compare(a.Rkey, b.Rkey) })
	targets := make([]placestream.MultistreamDefs_TargetView, 0, len(records))
	for _, record := range records {
		var target placestream.MultistreamTarget
		// Another app can write to the space, so one bad record must not hide the rest.
		if err := json.Unmarshal(record.Value, &target); err != nil {
			continue
		}
		targets = append(targets, placestream.MultistreamDefs_TargetView{
			Uri:    TargetURI(did, record.Rkey),
			Cid:    record.CID,
			Record: &glex.LexiconTypeDecoder{Val: &target},
		})
	}
	return targets, nil
}

//go:embed lexicons/*.json
var overlay embed.FS

// LexiconOverlay returns raw schema documents, replacing matching published NSIDs.
func LexiconOverlay() (map[string]json.RawMessage, error) {
	entries, err := overlay.ReadDir("lexicons")
	if err != nil {
		return nil, err
	}
	docs := make(map[string]json.RawMessage, len(entries))
	for _, entry := range entries {
		bs, err := overlay.ReadFile("lexicons/" + entry.Name())
		if err != nil {
			return nil, err
		}
		var doc struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(bs, &doc); err != nil {
			return nil, fmt.Errorf("decode space lexicon %s: %w", entry.Name(), err)
		}
		docs[doc.ID] = json.RawMessage(bs)
	}
	return docs, nil
}
