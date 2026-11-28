// Package spacestest has an in-memory personal space for tests.
package spacestest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
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
	Calls       []string
	SpaceExists bool
	ops         []map[string]any
}

func (f *Fake) Do(_ context.Context, _, _, method string, params map[string]any, body, out any) error {
	f.Calls = append(f.Calls, method)
	if f.Err != nil {
		return f.Err
	}
	var in struct {
		Space      string          `json:"space"`
		Repo       string          `json:"repo"`
		Collection string          `json:"collection"`
		Rkey       string          `json:"rkey"`
		Record     json.RawMessage `json:"record"`
		Writes     []struct {
			Type       string          `json:"$type"`
			Collection string          `json:"collection"`
			Rkey       string          `json:"rkey"`
			Value      json.RawMessage `json:"value"`
		} `json:"writes"`
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
	case "com.atproto.space.getSpace":
		if !f.SpaceExists && f.Rev == 0 {
			return rpcError("SpaceNotFound")
		}
		res = map[string]any{"space": params["space"]}
	case "com.atproto.space.createSpace":
		f.SpaceExists = true
	case "com.atproto.space.getRecord":
		rkey, _ := params["rkey"].(string)
		value, ok := f.Records[rkey]
		if !ok {
			return rpcError("RecordNotFound")
		}
		res = map[string]any{"value": value, "cid": "cid-" + rkey}
	case "com.atproto.space.createRecord", "com.atproto.space.putRecord":
		if method == "com.atproto.space.createRecord" {
			if !f.SpaceExists && f.Rev == 0 {
				return rpcError("SpaceNotFound")
			}
			if _, ok := f.Records[in.Rkey]; ok {
				return rpcError("RecordAlreadyExists")
			}
		}
		f.write(in.Collection, in.Rkey, in.Record)
		uri := strings.Join([]string{in.Space, in.Repo, in.Collection, in.Rkey}, "/")
		res = map[string]any{"uri": uri, "cid": fmt.Sprintf("cid-%d", f.Rev)}
	case "com.atproto.space.deleteRecord":
		f.write(in.Collection, in.Rkey, nil)
	case "com.atproto.space.applyWrites":
		if len(in.Writes) != 1 {
			return fmt.Errorf("expected one write")
		}
		w := in.Writes[0]
		if _, ok := f.Records[w.Rkey]; !ok {
			return rpcError("RecordNotFound")
		}
		switch w.Type {
		case "com.atproto.space.applyWrites#update":
			if len(w.Value) == 0 {
				return fmt.Errorf("missing update value")
			}
			f.write(w.Collection, w.Rkey, w.Value)
		case "com.atproto.space.applyWrites#delete":
			f.write(w.Collection, w.Rkey, nil)
		default:
			return fmt.Errorf("unexpected write %s", w.Type)
		}
	case "com.atproto.space.listRepoOps":
		since, _ := params["since"].(string)
		var ops []map[string]any
		for _, op := range f.ops {
			if op["rev"].(string) > since {
				ops = append(ops, op)
			}
		}
		res = map[string]any{"ops": ops}
	case "com.atproto.space.listRecords":
		records := []map[string]any{}
		for rkey, value := range f.Records {
			records = append(records, map[string]any{"collection": params["collection"], "rkey": rkey, "cid": "cid-" + rkey, "value": value})
		}
		res = map[string]any{"records": records}
	case "com.atproto.space.getLatestCommit":
		if f.Rev == 0 {
			return rpcError("RepoNotFound")
		}
		res = map[string]any{"commit": map[string]any{"rev": syntax.NewTIDFromInteger(uint64(f.Rev)).String()}}
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

func (f *Fake) write(collection, rkey string, value json.RawMessage) {
	if value == nil {
		delete(f.Records, rkey)
	} else {
		if f.Records == nil {
			f.Records = map[string]json.RawMessage{}
		}
		f.Records[rkey] = value
	}
	f.Rev++
	f.ops = append(f.ops, map[string]any{
		"rev": syntax.NewTIDFromInteger(uint64(f.Rev)).String(), "collection": collection, "rkey": rkey,
	})
}

func rpcError(name string) error {
	return &xrpc.Error{StatusCode: 400, Wrapped: &xrpc.XRPCError{ErrStr: name}}
}
