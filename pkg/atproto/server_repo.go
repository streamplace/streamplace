package atproto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/carstore"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/models"
	atrepo "github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/util"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
	cbg "github.com/whyrusleeping/cbor-gen"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
)

var ServerRepo *atrepo.Repo
var ServerPubMultibase string
var ServerCarStore carstore.CarStore
var ServerRepoUser models.Uid = models.Uid(1)

var serverRepoLock sync.Mutex
var serverCommitDB *gorm.DB
var serverRepoSigner func(ctx context.Context, did string, sb []byte) ([]byte, error)

// serverCommitSubscribers is notified when new commit events are created.
var serverCommitSubscribers []chan *ServerCommitEvent
var serverCommitSubLock sync.Mutex

// SubscribeServerCommits returns a channel that receives new commit events.
// Call UnsubscribeServerCommits to clean up.
func SubscribeServerCommits() chan *ServerCommitEvent {
	serverCommitSubLock.Lock()
	defer serverCommitSubLock.Unlock()
	ch := make(chan *ServerCommitEvent, 64)
	serverCommitSubscribers = append(serverCommitSubscribers, ch)
	return ch
}

// UnsubscribeServerCommits removes a subscriber channel.
func UnsubscribeServerCommits(ch chan *ServerCommitEvent) {
	serverCommitSubLock.Lock()
	defer serverCommitSubLock.Unlock()
	for i, sub := range serverCommitSubscribers {
		if sub == ch {
			serverCommitSubscribers = append(serverCommitSubscribers[:i], serverCommitSubscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

func notifyServerCommitSubscribers(event *ServerCommitEvent) {
	serverCommitSubLock.Lock()
	defer serverCommitSubLock.Unlock()
	for _, ch := range serverCommitSubscribers {
		select {
		case ch <- event:
		default:
			// Subscriber is slow, drop the event
		}
	}
}

// ServerCommitEvent stores commit events in local SQLite for relay replay (~72h TTL).
type ServerCommitEvent struct {
	Seq       int64     `gorm:"primaryKey;autoIncrement"`
	RepoDID   string    `gorm:"index:idx_server_repo_seq;column:repo_did"`
	Timestamp time.Time `gorm:"column:timestamp"`
	Data      []byte    `gorm:"column:data"`
	CID       string    `gorm:"column:cid"`
}

type serverRepoCloser struct {
	carStore *carstore.SQLiteStore
}

func (c *serverRepoCloser) Close() error {
	return c.carStore.Close()
}

func MakeServerRepo(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB) (Closer, error) {
	ctx = log.WithLogValues(ctx, "func", "MakeServerRepo")

	// Ensure data directory exists
	csPath := cli.DataFilePath([]string{"server-repo", "carstore.db"})
	if err := os.MkdirAll(filepath.Dir(csPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create server-repo directory: %w", err)
	}

	// Open file-backed carstore
	sqliteStore := &carstore.SQLiteStore{}
	err := sqliteStore.Open(csPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open server carstore at %s: %w", csPath, err)
	}
	ServerCarStore = sqliteStore

	// Generate or load key (namespaced by server host)
	configKey := fmt.Sprintf("server-repo-key:%s", cli.ServerHost)
	var priv *atcrypto.PrivateKeyK256
	keyBs, err := state.GetConfig(configKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get server repo key config: %w", err)
	}
	if keyBs != nil {
		priv, err = atcrypto.ParsePrivateBytesK256(keyBs.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse server repo key: %w", err)
		}
	} else {
		priv, err = atcrypto.GeneratePrivateKeyK256()
		if err != nil {
			return nil, fmt.Errorf("failed to generate server repo key: %w", err)
		}
		err = state.PutConfig(configKey, priv.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to save server repo key: %w", err)
		}
	}

	pub, err := priv.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get server repo public key: %w", err)
	}
	ServerPubMultibase = pub.Multibase()

	serverRepoSigner = func(ctx context.Context, did string, sb []byte) ([]byte, error) {
		return priv.HashAndSign(sb)
	}

	// Open or create the repo from carstore
	head, err := ServerCarStore.GetUserRepoHead(ctx, ServerRepoUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get server repo head: %w", err)
	}

	if head == cid.Undef {
		log.Log(ctx, "no existing server repo, creating new one")
		ses, err := ServerCarStore.NewDeltaSession(ctx, ServerRepoUser, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create delta session for new server repo: %w", err)
		}
		ServerRepo = atrepo.NewRepo(ctx, cli.ServerDID(), ses)

		// Do an initial commit so the repo has a valid head
		root, rev, err := ServerRepo.Commit(ctx, serverRepoSigner)
		if err != nil {
			return nil, fmt.Errorf("failed to initial commit server repo: %w", err)
		}
		_, err = ses.CloseWithRoot(ctx, root, rev)
		if err != nil {
			return nil, fmt.Errorf("failed to close initial server repo session: %w", err)
		}
		log.Log(ctx, "created new server repo", "did", cli.ServerDID(), "root", root.String(), "rev", rev)
	} else {
		log.Log(ctx, "opening existing server repo", "head", head.String())
		ses, err := ServerCarStore.NewDeltaSession(ctx, ServerRepoUser, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create delta session for existing server repo: %w", err)
		}
		ServerRepo, err = atrepo.OpenRepo(ctx, ses, head)
		if err != nil {
			return nil, fmt.Errorf("failed to open existing server repo: %w", err)
		}
	}

	// Open local SQLite for commit events
	commitDBPath := cli.DataFilePath([]string{"server-repo", "commits.db"})
	db, err := gorm.Open(sqlite.Open(commitDBPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open server commit db at %s: %w", commitDBPath, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	err = db.AutoMigrate(&ServerCommitEvent{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate server commit events: %w", err)
	}
	serverCommitDB = db

	// Prune old commit events (>72h)
	cutoff := time.Now().Add(-72 * time.Hour)
	if err := serverCommitDB.Where("timestamp < ?", cutoff).Delete(&ServerCommitEvent{}).Error; err != nil {
		log.Warn(ctx, "failed to prune old server commit events", "error", err)
	}

	return &serverRepoCloser{carStore: sqliteStore}, nil
}

func OpenServerRepo(ctx context.Context) (*atrepo.Repo, *carstore.DeltaSession, error) {
	ses, err := ServerCarStore.NewDeltaSession(ctx, ServerRepoUser, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("OpenServerRepo: failed to create delta session: %w", err)
	}

	base := ses.BaseCid()
	if base == cid.Undef {
		return nil, nil, fmt.Errorf("OpenServerRepo: delta session has no base cid")
	}

	r, err := atrepo.OpenRepo(ctx, ses, base)
	if err != nil {
		return nil, nil, fmt.Errorf("OpenServerRepo: failed to open repo: %w", err)
	}
	return r, ses, nil
}

// CreateServerCommitEvent stores a commit event in the local commit DB for relay replay.
func CreateServerCommitEvent(commit *comatproto.SyncSubscribeRepos_Commit, signedData string) error {
	// Get previous seq
	var prev ServerCommitEvent
	err := serverCommitDB.Order("seq DESC").Limit(1).First(&prev).Error
	var seq int64 = 1
	if err == nil {
		seq = prev.Seq + 1
	}

	commit.Seq = seq

	buf := bytes.Buffer{}
	err = commit.MarshalCBOR(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal commit event: %w", err)
	}
	timestamp, err := time.Parse(util.ISO8601, commit.Time)
	if err != nil {
		return fmt.Errorf("failed to parse commit time: %w", err)
	}
	event := &ServerCommitEvent{
		RepoDID:   commit.Repo,
		Timestamp: timestamp.UTC(),
		Data:      buf.Bytes(),
		CID:       commit.Commit.String(),
	}
	if err := serverCommitDB.Create(event).Error; err != nil {
		return err
	}
	notifyServerCommitSubscribers(event)
	return nil
}

// GetServerCommitEventsSinceSeq returns commit events after the given seq for relay replay.
func GetServerCommitEventsSinceSeq(repoDID string, seq int64) ([]*ServerCommitEvent, error) {
	var events []*ServerCommitEvent
	err := serverCommitDB.Where("repo_did = ? AND seq > ?", repoDID, seq).
		Order("seq ASC").Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

// ToCommitEvent converts a ServerCommitEvent to a SyncSubscribeRepos_Commit.
func (ev *ServerCommitEvent) ToCommitEvent() (*comatproto.SyncSubscribeRepos_Commit, error) {
	commit := &comatproto.SyncSubscribeRepos_Commit{}
	err := commit.UnmarshalCBOR(bytes.NewReader(ev.Data))
	if err != nil {
		return nil, err
	}
	return commit, nil
}

// CommitServerRepoRecord puts or updates a record in the server repo and stores the commit event.
func CommitServerRepoRecord(ctx context.Context, cli *config.CLI, collection string, rkey string, value cbg.CBORMarshaler) error {
	serverRepoLock.Lock()
	defer serverRepoLock.Unlock()

	ses, err := ServerCarStore.NewDeltaSession(ctx, ServerRepoUser, nil)
	if err != nil {
		return fmt.Errorf("CommitServerRepoRecord: failed to create delta session: %w", err)
	}

	head, err := ServerCarStore.GetUserRepoHead(ctx, ServerRepoUser)
	if err != nil {
		return fmt.Errorf("CommitServerRepoRecord: failed to get repo head: %w", err)
	}

	r, err := atrepo.OpenRepo(ctx, ses, head)
	if err != nil {
		return fmt.Errorf("CommitServerRepoRecord: failed to open repo: %w", err)
	}

	rpath := fmt.Sprintf("%s/%s", collection, rkey)
	var recordCid cid.Cid
	var action string
	_, _, err = r.GetRecord(ctx, rpath)
	if err != nil {
		// Record doesn't exist, create it
		recordCid, err = r.PutRecord(ctx, rpath, value)
		if err != nil {
			return fmt.Errorf("CommitServerRepoRecord: failed to put record: %w", err)
		}
		action = ActionCreate
	} else {
		// Record exists, update it
		recordCid, err = r.UpdateRecord(ctx, rpath, value)
		if err != nil {
			return fmt.Errorf("CommitServerRepoRecord: failed to update record: %w", err)
		}
		action = ActionUpdate
	}

	root, rev, err := r.Commit(ctx, serverRepoSigner)
	if err != nil {
		return fmt.Errorf("CommitServerRepoRecord: failed to commit: %w", err)
	}

	blocks, err := ses.CloseWithRoot(ctx, root, rev)
	if err != nil {
		return fmt.Errorf("CommitServerRepoRecord: failed to close session: %w", err)
	}

	ServerRepo = r

	cidLink := lexutil.LexLink(recordCid)
	signed := r.SignedCommit()
	commit := &comatproto.SyncSubscribeRepos_Commit{
		Repo:   cli.ServerDID(),
		Blocks: blocks,
		Rev:    rev,
		Commit: lexutil.LexLink(root),
		Time:   time.Now().Format(util.ISO8601),
		Ops: []*comatproto.SyncSubscribeRepos_RepoOp{
			{
				Action: action,
				Path:   rpath,
				Cid:    &cidLink,
			},
		},
		TooBig: false,
	}
	err = CreateServerCommitEvent(commit, signed.Data.String())
	if err != nil {
		return fmt.Errorf("CommitServerRepoRecord: failed to create commit event: %w", err)
	}

	return nil
}

// Server repo query functions

func ServerRepoMerkleProof(ctx context.Context, collection string, rkey string) ([]byte, error) {
	serverRepoLock.Lock()
	defer serverRepoLock.Unlock()

	_, robs, err := OpenServerRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to open repo: %w", err)
	}

	bs := util.NewLoggingBstore(robs)

	root, err := ServerCarStore.GetUserRepoHead(ctx, ServerRepoUser)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to get user repo head: %w", err)
	}

	r, err := atrepo.OpenRepo(ctx, bs, root)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to open repo: %w", err)
	}

	_, _, err = r.GetRecordBytes(ctx, collection+"/"+rkey)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to get record bytes: %w", err)
	}

	blocks := bs.GetLoggedBlocks()

	buf := new(bytes.Buffer)
	hb, err := cbor.DumpObject(&car.CarHeader{
		Roots:   []cid.Cid{root},
		Version: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to dump car header: %w", err)
	}
	if _, err := carstore.LdWrite(buf, hb); err != nil {
		return nil, err
	}

	for _, blk := range blocks {
		if _, err := carstore.LdWrite(buf, blk.Cid().Bytes(), blk.RawData()); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// ServerRepoListCollections walks the server repo's MST and returns
// the distinct collection NSIDs currently holding at least one record.
// Used by com.atproto.repo.describeRepo to advertise what's actually
// in the repo rather than a hardcoded list. Returned collections are
// sorted lexicographically for stable output.
func ServerRepoListCollections(ctx context.Context) ([]string, error) {
	serverRepoLock.Lock()
	defer serverRepoLock.Unlock()

	r, _, err := OpenServerRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoListCollections: failed to open repo: %w", err)
	}
	seen := map[string]struct{}{}
	err = r.ForEach(ctx, "", func(rpath string, _ cid.Cid) error {
		// rpath is "<collection>/<rkey>"; pull the prefix.
		slash := strings.IndexByte(rpath, '/')
		if slash <= 0 {
			return nil
		}
		seen[rpath[:slash]] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ServerRepoListCollections: error iterating records: %w", err)
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out, nil
}

// defaultListRecordsLimit / maxListRecordsLimit mirror the standard
// com.atproto.repo.listRecords lexicon defaults so callers get
// predictable pagination regardless of whether they hit a Streamplace
// node or a stock PDS.
const (
	defaultListRecordsLimit = 50
	maxListRecordsLimit     = 100
)

// ServerRepoListRecords returns records in a single collection of the
// server's atproto repo, honoring the standard listRecords contract:
//
//   - filters strictly to the requested `collection`
//   - paginates via `cursor` (opaque to the client; we use the last
//     rkey of the prior page) and `limit` (clamped to [1, 100],
//     defaulting to 50)
//   - reverses the output slice when `reverse` is true; the natural
//     MST walk order is lex-ascending rkey, which for TID-shaped
//     rkeys is chronologically oldest-first
//
// `repo` is used only to build the `at://<repo>/<collection>/<rkey>`
// URIs in the response; the underlying repo is always the server's own.
func ServerRepoListRecords(ctx context.Context, collection string, cursor string, limit int, repo string, reverse *bool) (*comatproto.RepoListRecords_Output, error) {
	serverRepoLock.Lock()
	defer serverRepoLock.Unlock()

	r, ses, err := OpenServerRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoListRecords: failed to open repo: %w", err)
	}

	if limit <= 0 {
		limit = defaultListRecordsLimit
	}
	if limit > maxListRecordsLimit {
		limit = maxListRecordsLimit
	}

	prefix := collection + "/"
	out := &comatproto.RepoListRecords_Output{
		Records: []*comatproto.RepoListRecords_Record{},
	}
	var lastRkey string
	err = r.ForEach(ctx, prefix, func(rpath string, c cid.Cid) error {
		// ForEach walks lex-ascending from the prefix. As soon as we
		// see a key that doesn't carry the prefix we're past the end
		// of the requested collection — stop the walk.
		if !strings.HasPrefix(rpath, prefix) {
			return atrepo.ErrDoneIterating
		}
		rkey := strings.TrimPrefix(rpath, prefix)
		// Cursor is the rkey of the last record on the previous page;
		// skip until we're strictly past it.
		if cursor != "" && rkey <= cursor {
			return nil
		}
		if len(out.Records) >= limit {
			return atrepo.ErrDoneIterating
		}
		raw, err := getBlock(ctx, ses, c)
		if err != nil {
			return fmt.Errorf("ServerRepoListRecords: %w", err)
		}
		val, err := lexutil.CborDecodeValue(raw)
		if err != nil {
			return fmt.Errorf("ServerRepoListRecords: failed to decode record for rkey %q: %w", rkey, err)
		}
		out.Records = append(out.Records, &comatproto.RepoListRecords_Record{
			Uri:   fmt.Sprintf("at://%s/%s", repo, rpath),
			Cid:   c.String(),
			Value: &lexutil.LexiconTypeDecoder{Val: val},
		})
		lastRkey = rkey
		return nil
	})
	// Repo.ForEach compares the underlying mst walker's error against
	// atrepo.ErrDoneIterating with `==`, but the walker wraps callback
	// errors with `%w` before bubbling them up — so the sentinel never
	// matches the equality check. Use errors.Is on our side so the
	// early-exit path is observable through the wrapped error.
	if err != nil && !errors.Is(err, atrepo.ErrDoneIterating) {
		return nil, fmt.Errorf("ServerRepoListRecords: error iterating records: %w", err)
	}

	// Surface a cursor whenever we returned a full page; clients can
	// pass it back to fetch the next page. If we returned fewer than
	// limit records there are definitely no more, so omit it.
	if len(out.Records) == limit && lastRkey != "" {
		cur := lastRkey
		out.Cursor = &cur
	}

	if reverse != nil && *reverse {
		for i, j := 0, len(out.Records)-1; i < j; i, j = i+1, j-1 {
			out.Records[i], out.Records[j] = out.Records[j], out.Records[i]
		}
	}

	return out, nil
}

func ServerRepoGetRecord(ctx context.Context, repo string, collection string, rkey string) (*comatproto.RepoGetRecord_Output, error) {
	serverRepoLock.Lock()
	defer serverRepoLock.Unlock()

	r, ses, err := OpenServerRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoGetRecord: failed to open repo: %w", err)
	}
	outCID, _, err := r.GetRecord(ctx, fmt.Sprintf("%s/%s", collection, rkey))
	if err != nil {
		return nil, err
	}
	raw, err := getBlock(ctx, ses, outCID)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoGetRecord: %w", err)
	}
	rec, err := lexutil.CborDecodeValue(raw)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoGetRecord: failed to decode record: %w", err)
	}
	str := outCID.String()
	return &comatproto.RepoGetRecord_Output{
		Uri:   fmt.Sprintf("at://%s/%s/%s", repo, collection, rkey),
		Cid:   &str,
		Value: &lexutil.LexiconTypeDecoder{Val: rec},
	}, nil
}

func ServerRepoGetRepo(ctx context.Context, since string) ([]byte, error) {
	serverRepoLock.Lock()
	defer serverRepoLock.Unlock()

	_, robs, err := OpenServerRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to open repo: %w", err)
	}

	bs := util.NewLoggingBstore(robs)

	root, err := ServerCarStore.GetUserRepoHead(ctx, ServerRepoUser)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to get user repo head: %w", err)
	}

	r, err := atrepo.OpenRepo(ctx, bs, root)
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to open repo: %w", err)
	}

	err = r.ForEach(ctx, "", func(rkey string, c cid.Cid) error {
		_, _, err = r.GetRecordBytes(ctx, rkey)
		if err != nil {
			return fmt.Errorf("ServerRepoMerkleProof: failed to get record bytes: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error doing foreach: %w", err)
	}

	blocks := bs.GetLoggedBlocks()

	buf := new(bytes.Buffer)
	hb, err := cbor.DumpObject(&car.CarHeader{
		Roots:   []cid.Cid{root},
		Version: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("ServerRepoMerkleProof: failed to dump car header: %w", err)
	}
	if _, err := carstore.LdWrite(buf, hb); err != nil {
		return nil, err
	}

	for _, blk := range blocks {
		if _, err := carstore.LdWrite(buf, blk.Cid().Bytes(), blk.RawData()); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func getBlock(ctx context.Context, ses *carstore.DeltaSession, c cid.Cid) ([]byte, error) {
	b, err := ses.Get(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", c.String(), err)
	}
	return b.RawData(), nil
}
