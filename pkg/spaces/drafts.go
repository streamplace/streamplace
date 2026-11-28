package spaces

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/placestream"
)

const DraftMetadataCollection = "place.stream.vod.draftMetadata"

// CollectionGrant checks the expanded PDS grant. A cutover needs read and all three
// record writes; a grant for multistream alone must not trigger it.
func CollectionGrant(scope, did, collection string) bool {
	return ParseCollectionPermissions(scope, did, collection).Full()
}

// CollectionActionsGrant checks that expanded PDS permissions cover every requested action.
func CollectionActionsGrant(scope, did, collection string, actions ...string) bool {
	return ParseCollectionPermissions(scope, did, collection).Allows(actions...)
}

type CollectionPermissions uint8

const (
	readPermission CollectionPermissions = 1 << iota
	createPermission
	updatePermission
	deletePermission
	allPermissions = readPermission | createPermission | updatePermission | deletePermission
)

func actionPermission(action string) CollectionPermissions {
	switch action {
	case "read", "read_self":
		return readPermission
	case "create":
		return createPermission
	case "update":
		return updatePermission
	case "delete":
		return deletePermission
	default:
		return 0
	}
}

func (p CollectionPermissions) Full() bool { return p == allPermissions }

func (p CollectionPermissions) Allows(actions ...string) bool {
	for _, action := range actions {
		if p&actionPermission(action) == 0 {
			return false
		}
	}
	return len(actions) > 0
}

// ParseCollectionPermissions resolves grants for just this DID and collection;
// a zero value means the token has no applicable private-space permission.
func ParseCollectionPermissions(scope, did, collection string) CollectionPermissions {
	if _, err := syntax.ParseDID(did); err != nil || collection == "" {
		return 0
	}
	var allowed CollectionPermissions
tokens:
	for token := range strings.FieldsSeq(scope) {
		target, ok := strings.CutPrefix(token, "space:")
		if !ok {
			continue
		}
		spaceType, query, _ := strings.Cut(target, "?")
		if spaceType != PersonalSpaceType && spaceType != "*" {
			continue
		}
		q, err := url.ParseQuery(query)
		if err != nil || len(q["authority"]) != 1 || (q.Get("authority") != did && q.Get("authority") != "*") {
			continue
		}
		if keys := q["skey"]; len(keys) > 1 || (len(keys) == 1 && keys[0] != PersonalSpaceKey && keys[0] != "*") {
			continue
		}
		collections := q["collection"]
		if slices.Contains(collections, "") || (!slices.Contains(collections, collection) && !slices.Contains(collections, "*")) {
			continue
		}
		if !q.Has("action") {
			return allPermissions // Alpha defaults to read, create, update, delete.
		}
		var granted CollectionPermissions
		for _, action := range q["action"] {
			bit := actionPermission(action)
			if bit == 0 {
				continue tokens
			}
			granted |= bit
		}
		allowed |= granted
		if allowed.Full() {
			break
		}
	}
	return allowed
}

func DraftAuthorizationError(err error) bool {
	var rpcErr *xrpc.Error
	return (errors.As(err, &rpcErr) && (rpcErr.StatusCode == http.StatusUnauthorized || rpcErr.StatusCode == http.StatusForbidden)) ||
		spaceError(err, "insufficient_scope")
}

func draftSpaceExists(ctx context.Context, c Client, did string) (bool, error) {
	params := map[string]any{"space": PersonalSpaceURI(did)}
	err := c.Do(ctx, "GET", "", "com.atproto.space.getSpace", params, nil, nil)
	if spaceError(err, "SpaceNotFound") {
		return false, nil
	}
	return err == nil, err
}

func EnsureDraftSpace(ctx context.Context, c Client, did string) error {
	exists, err := draftSpaceExists(ctx, c, did)
	if err == nil && !exists {
		err = createPersonalSpace(ctx, c)
	}
	return err
}

// CreateDraftMetadata never overwrites. The caller decides whether an existing
// record wins (migration) or is an error (a newly allocated draft).
func CreateDraftMetadata(ctx context.Context, c Client, did, rkey string, metadata *placestream.VodDraftMetadata) (created bool, err error) {
	body := struct {
		recordAddress
		Record *placestream.VodDraftMetadata `json:"record"`
	}{recordAddress{PersonalSpaceURI(did), did, DraftMetadataCollection, rkey}, metadata}
	err = c.Do(ctx, "POST", "application/json", "com.atproto.space.createRecord", nil, &body, nil)
	if spaceError(err, "RecordAlreadyExists") {
		return false, nil
	}
	return err == nil, err
}

func GetDraftMetadata(ctx context.Context, c Client, did, rkey string) (*Record, error) {
	params := map[string]any{"space": PersonalSpaceURI(did), "repo": did, "collection": DraftMetadataCollection, "rkey": rkey}
	out := Record{Rkey: rkey}
	err := c.Do(ctx, "GET", "", "com.atproto.space.getRecord", params, nil, &out)
	if noRepo(err) || spaceError(err, "RecordNotFound") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// WriteExistingDraftMetadata updates, or deletes if metadata is nil. Alpha
// putRecord is an upsert; applyWrites enforces existence at commit time.
func WriteExistingDraftMetadata(ctx context.Context, c Client, did, rkey string, metadata *placestream.VodDraftMetadata) (bool, error) {
	type write struct {
		Type       string                        `json:"$type"`
		Collection string                        `json:"collection"`
		Rkey       string                        `json:"rkey"`
		Value      *placestream.VodDraftMetadata `json:"value,omitempty"`
	}
	kind := "update"
	if metadata == nil {
		kind = "delete"
	}
	body := struct {
		Space  string   `json:"space"`
		Repo   string   `json:"repo"`
		Writes [1]write `json:"writes"`
	}{PersonalSpaceURI(did), did, [1]write{{"com.atproto.space.applyWrites#" + kind, DraftMetadataCollection, rkey, metadata}}}
	err := c.Do(ctx, "POST", "application/json", "com.atproto.space.applyWrites", nil, &body, nil)
	if noRepo(err) || spaceError(err, "RecordNotFound") {
		return false, nil
	}
	return err == nil, err
}

// DraftCreateObserved reconciles an interrupted create without blindly
// recreating a record the owner may have deleted after the PDS committed it.
func DraftCreateObserved(ctx context.Context, c Client, did, rkey, before string) (bool, error) {
	record, err := GetDraftMetadata(ctx, c, did, rkey)
	if err != nil || record != nil {
		return record != nil, err
	}
	rev, err := LatestRev(ctx, c, did)
	if err != nil {
		return false, err
	}
	if rev == "" {
		if before != "" {
			return true, nil // The repo was removed.
		}
		exists, err := draftSpaceExists(ctx, c, did)
		return !exists, err // The space existed before the create was attempted.
	}
	if rev == before {
		return false, nil
	}

	params := map[string]any{"space": PersonalSpaceURI(did), "repo": did, "limit": 1000, "excludeValues": true}
	if before != "" {
		tid, err := syntax.ParseTID(before)
		if err != nil || tid.Integer() == 0 {
			return false, fmt.Errorf("invalid pre-create space revision")
		}
		// Include the old head to detect a compacted or replaced oplog.
		params["since"] = syntax.NewTIDFromInteger(tid.Integer() - 1).String()
	}
	baseline := false
	for {
		var page struct {
			Cursor string `json:"cursor"`
			Ops    []struct {
				Rev        string `json:"rev"`
				Collection string `json:"collection"`
				Rkey       string `json:"rkey"`
			} `json:"ops"`
		}
		if err := c.Do(ctx, "GET", "", "com.atproto.space.listRepoOps", params, nil, &page); err != nil {
			return false, err
		}
		for _, op := range page.Ops {
			if op.Collection == DraftMetadataCollection && op.Rkey == rkey {
				return true, nil
			}
			baseline = baseline || op.Rev == before
		}
		if page.Cursor == "" {
			if baseline {
				return false, nil
			}
			return false, fmt.Errorf("cannot verify interrupted draft migration against space history")
		}
		params["cursor"] = page.Cursor
	}
}
