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
	"stream.place/streamplace/pkg/spid"
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

type draftWrite struct {
	Type       string                        `json:"$type"`
	Collection string                        `json:"collection"`
	Rkey       string                        `json:"rkey"`
	Value      *placestream.VodDraftMetadata `json:"value,omitempty"`
}

func applyDraftWrites(ctx context.Context, c Client, did string, writes []draftWrite) error {
	body := struct {
		Space  string       `json:"space"`
		Repo   string       `json:"repo"`
		Writes []draftWrite `json:"writes"`
	}{PersonalSpaceURI(did), did, writes}
	return c.Do(ctx, "POST", "application/json", "com.atproto.space.applyWrites", nil, &body, nil)
}

// DraftCreateBaseline anchors the first migration write without leaving a record.
func DraftCreateBaseline(ctx context.Context, c Client, did string) (string, error) {
	rev, err := LatestRev(ctx, c, did)
	if err != nil || rev != "" {
		return rev, err
	}
	if err := EnsureDraftSpace(ctx, c, did); err != nil {
		return "", err
	}
	// Alpha may discard an oplog prefix without reporting it. A create/delete
	// batch at a fresh key leaves an oplog revision but no live metadata.
	key := spid.TIDClock.Next().String()
	if err := applyDraftWrites(ctx, c, did, []draftWrite{
		{"com.atproto.space.applyWrites#create", DraftMetadataCollection, key, &placestream.VodDraftMetadata{}},
		{"com.atproto.space.applyWrites#delete", DraftMetadataCollection, key, nil},
	}); err != nil {
		return "", err
	}
	rev, err = LatestRev(ctx, c, did)
	if err == nil && rev == "" {
		err = errors.New("missing space revision after draft migration checkpoint")
	}
	return rev, err
}

type draftOpsPage struct {
	Cursor string `json:"cursor"`
	Commit struct {
		Rev string `json:"rev"`
	} `json:"commit"`
	Ops []struct {
		Rev        string `json:"rev"`
		Collection string `json:"collection"`
		Rkey       string `json:"rkey"`
	} `json:"ops"`
}

func listDraftOps(ctx context.Context, c Client, params map[string]any) (draftOpsPage, error) {
	var page draftOpsPage
	err := c.Do(ctx, "GET", "", "com.atproto.space.listRepoOps", params, nil, &page)
	return page, err
}

var errDraftCreateHistory = errors.New("cannot verify interrupted draft migration against space history")

// WriteExistingDraftMetadata updates, or deletes if metadata is nil. Alpha
// putRecord is an upsert; applyWrites enforces existence at commit time.
func WriteExistingDraftMetadata(ctx context.Context, c Client, did, rkey string, metadata *placestream.VodDraftMetadata) (bool, error) {
	kind := "update"
	if metadata == nil {
		kind = "delete"
	}
	err := applyDraftWrites(ctx, c, did, []draftWrite{{"com.atproto.space.applyWrites#" + kind, DraftMetadataCollection, rkey, metadata}})
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
		exists, err := draftSpaceExists(ctx, c, did)
		if err != nil || !exists {
			return !exists, err
		}
		return false, errDraftCreateHistory
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
	baseline, last := false, ""
	for {
		page, err := listDraftOps(ctx, c, params)
		if err != nil {
			return false, err
		}
		for _, op := range page.Ops {
			if op.Collection == DraftMetadataCollection && op.Rkey == rkey {
				return true, nil
			}
			baseline = baseline || op.Rev == before
			last = op.Rev
		}
		if page.Commit.Rev != rev {
			return false, errDraftCreateHistory
		}
		if page.Cursor == "" {
			break
		}
		if page.Cursor == params["cursor"] {
			return false, errDraftCreateHistory
		}
		params["cursor"] = page.Cursor
	}
	if !baseline || last != rev {
		return false, errDraftCreateHistory
	}
	// Prefix pruning need not change the head. Recheck the baseline if the
	// scan spanned multiple snapshots.
	if _, paged := params["cursor"]; paged {
		delete(params, "cursor")
		params["limit"] = 1
		page, err := listDraftOps(ctx, c, params)
		if err != nil {
			return false, err
		}
		if page.Commit.Rev != rev || len(page.Ops) == 0 || page.Ops[0].Rev != before {
			return false, errDraftCreateHistory
		}
	}
	return false, nil
}
