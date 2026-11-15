// Package spacestest has an in-memory personal space for tests.
package spacestest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bluesky-social/indigo/xrpc"
)

// Fake is one account's personal space, holding one collection's records by rkey.
type Fake struct {
	Records map[string]json.RawMessage
	// Rev counts the writes; the space has no repo before the first.
	Rev int
	// Err, when set, fails every call.
	Err error
	// Calls lists the methods called, in order.
	Calls []string
}

func (f *Fake) Do(_ context.Context, _, _, method string, params map[string]any, body, out any) error {
	f.Calls = append(f.Calls, method)
	if f.Err != nil {
		return f.Err
	}
	var in struct {
		Rkey   string          `json:"rkey"`
		Record json.RawMessage `json:"record"`
	}
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bs, &in); err != nil {
			return err
		}
	}
	var res any
	switch method {
	case "com.atproto.space.putRecord":
		if f.Records == nil {
			f.Records = map[string]json.RawMessage{}
		}
		f.Records[in.Rkey] = in.Record
		f.Rev++
		res = map[string]any{"uri": "at://space/" + in.Rkey, "cid": fmt.Sprintf("cid-%d", f.Rev)}
	case "com.atproto.space.deleteRecord":
		delete(f.Records, in.Rkey)
		f.Rev++
	case "com.atproto.space.listRecords":
		records := []map[string]any{}
		for rkey, value := range f.Records {
			records = append(records, map[string]any{"collection": params["collection"], "rkey": rkey, "cid": "cid-" + rkey, "value": value})
		}
		res = map[string]any{"records": records}
	case "com.atproto.space.getLatestCommit":
		if f.Rev == 0 {
			return &xrpc.Error{StatusCode: 400, Wrapped: &xrpc.XRPCError{ErrStr: "RepoNotFound"}}
		}
		res = map[string]any{"commit": map[string]any{"rev": fmt.Sprint(f.Rev)}}
	default:
		return fmt.Errorf("unexpected method %s", method)
	}
	if out == nil {
		return nil
	}
	bs, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, out)
}
