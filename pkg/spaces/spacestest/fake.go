// Package spacestest has an in-memory personal space for tests.
package spacestest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
	ops         []repoOp
}

type repoOp struct {
	Rev        string `json:"rev"`
	Collection string `json:"collection"`
	Rkey       string `json:"rkey"`
	index      int
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
		if !f.SpaceExists && f.Rev == 0 {
			return rpcError("SpaceNotFound")
		}
		staged := make(map[string]json.RawMessage, len(in.Writes))
		for _, w := range in.Writes {
			value, stagedAlready := staged[w.Rkey]
			if !stagedAlready {
				value = f.Records[w.Rkey]
			}
			switch w.Type {
			case "com.atproto.space.applyWrites#create":
				if value != nil {
					return rpcError("RecordAlreadyExists")
				}
				staged[w.Rkey] = w.Value
			case "com.atproto.space.applyWrites#update":
				if value == nil {
					return rpcError("RecordNotFound")
				}
				staged[w.Rkey] = w.Value
			case "com.atproto.space.applyWrites#delete":
				if value == nil {
					return rpcError("RecordNotFound")
				}
				staged[w.Rkey] = nil
			default:
				return fmt.Errorf("unexpected write %s", w.Type)
			}
		}
		if len(in.Writes) != 0 {
			f.Rev++
			for i, w := range in.Writes {
				f.applyOp(w.Collection, w.Rkey, w.Value, i)
			}
		}
	case "com.atproto.space.listRepoOps":
		since, _ := params["since"].(string)
		cursor, _ := params["cursor"].(string)
		cursorRev, index, _ := strings.Cut(cursor, "/")
		cursorIndex, _ := strconv.Atoi(index)
		limit, _ := params["limit"].(int)
		var ops []repoOp
		next := ""
		for _, op := range f.ops {
			if op.Rev <= since || op.Rev < cursorRev || (op.Rev == cursorRev && op.index <= cursorIndex) {
				continue
			}
			ops = append(ops, op)
			if len(ops) == limit {
				next = fmt.Sprintf("%s/%d", op.Rev, op.index)
				break
			}
		}
		res = map[string]any{"ops": ops, "cursor": next, "commit": map[string]any{"rev": f.revision()}}
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
		res = map[string]any{"commit": map[string]any{"rev": f.revision()}}
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
	f.Rev++
	f.applyOp(collection, rkey, value, 0)
}

func (f *Fake) revision() string {
	return syntax.NewTIDFromInteger(uint64(f.Rev)).String()
}

func (f *Fake) applyOp(collection, rkey string, value json.RawMessage, index int) {
	if value == nil {
		delete(f.Records, rkey)
	} else {
		if f.Records == nil {
			f.Records = map[string]json.RawMessage{}
		}
		f.Records[rkey] = value
	}
	f.ops = append(f.ops, repoOp{Rev: f.revision(), Collection: collection, Rkey: rkey, index: index})
}

func rpcError(name string) error {
	return &xrpc.Error{StatusCode: 400, Wrapped: &xrpc.XRPCError{ErrStr: name}}
}
