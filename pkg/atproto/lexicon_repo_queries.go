package atproto

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/bluesky-social/indigo/carstore"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/util"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
	"stream.place/streamplace/pkg/log"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
)

var repoLock sync.Mutex

func LexiconRepoMerkleProof(ctx context.Context, collection string, rkey string) ([]byte, error) {
	repoLock.Lock()
	defer repoLock.Unlock()

	_, robs, err := OpenLexiconRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to open repo: %w", err)
	}

	bs := util.NewLoggingBstore(robs)

	root, err := CarStore.GetUserRepoHead(ctx, RepoUser)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to get user repo head: %w", err)
	}

	log.Warn(ctx, "got root", "root", root.String())

	r, err := repo.OpenRepo(ctx, bs, root)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to open repo: %w", err)
	}

	_, _, err = r.GetRecordBytes(ctx, collection+"/"+rkey)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to get record bytes: %w", err)
	}

	blocks := bs.GetLoggedBlocks()

	buf := new(bytes.Buffer)
	hb, err := cbor.DumpObject(&car.CarHeader{
		Roots:   []cid.Cid{root},
		Version: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dump car header: %w", err)
	}
	if _, err := carstore.LdWrite(buf, hb); err != nil {
		return nil, err
	}

	for _, blk := range blocks {
		log.Warn(ctx, "writing block", "cid", blk.Cid().String(), "version", blk.Cid().Version())
		if _, err := carstore.LdWrite(buf, blk.Cid().Bytes(), blk.RawData()); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func LexiconRepoListRecords(ctx context.Context, collection string, cursor string, limit int, repo string, reverse *bool) (*comatproto.RepoListRecords_Output, error) {
	repoLock.Lock()
	defer repoLock.Unlock()

	r, ses, err := OpenLexiconRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoListRecords: failed to open repo: %w", err)
	}
	out := &comatproto.RepoListRecords_Output{
		Records: []*comatproto.RepoListRecords_Record{},
	}
	err = r.ForEach(ctx, "", func(rkey string, c cid.Cid) error {
		val, err := GetRecordCBOR(ctx, ses, c, collection, rkey)
		if err != nil {
			return fmt.Errorf("handleComAtprotoRepoListRecords: failed to get record for collection %q, rkey %q: %w", collection, rkey, err)
		}
		log.Warn(ctx, "got record", "rkey", rkey, "cid", c.String())
		out.Records = append(out.Records, &comatproto.RepoListRecords_Record{
			Uri:   fmt.Sprintf("at://%s/%s", repo, rkey),
			Cid:   c.String(),
			Value: &lexutil.LexiconTypeDecoder{Val: val},
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoListRecords: error iterating records for collection %q: %w", collection, err)
	}
	return out, nil
}

func LexiconRepoGetRecord(ctx context.Context, repo string, collection string, rkey string) (*comatproto.RepoGetRecord_Output, error) {
	repoLock.Lock()
	defer repoLock.Unlock()

	r, ses, err := OpenLexiconRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to open repo: %w", err)
	}
	outCID, _, err := r.GetRecord(ctx, fmt.Sprintf("%s/%s", collection, rkey))
	if err != nil {
		return nil, err
	}
	rec, err := GetRecordCBOR(ctx, ses, outCID, collection, rkey)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoGetRecord: failed to get record: %w", err)
	}
	str := outCID.String()
	return &comatproto.RepoGetRecord_Output{
		Uri:   fmt.Sprintf("at://%s/%s/%s", repo, collection, rkey),
		Cid:   &str,
		Value: &lexutil.LexiconTypeDecoder{Val: rec},
	}, nil
}
