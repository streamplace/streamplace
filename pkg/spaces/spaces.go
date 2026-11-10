package spaces

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bluesky-social/indigo/xrpc"
)

const (
	PersonalSpaceType = "place.stream.personal"
	PersonalSpaceKey  = "self"
)

// Client is the XRPC surface used by personal spaces.
type Client interface {
	Do(ctx context.Context, kind string, inpenc, method string, params map[string]any, bodyobj any, out any) error
}

func PersonalSpaceURI(did string) string {
	return "at://" + did + "/space/" + PersonalSpaceType + "/" + PersonalSpaceKey
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
