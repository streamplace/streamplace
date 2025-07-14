package atproto

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/carstore"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/models"
	"github.com/bluesky-social/indigo/mst"
	atrepo "github.com/bluesky-social/indigo/repo"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/whyrusleeping/go-did"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

var LexiconRepo *atrepo.Repo
var LexiconPubMultibase string
var RepoUser models.Uid = models.Uid(1)
var CarStore carstore.CarStore

func walkLexicons(ctx context.Context, bundle embed.FS, path string) ([][]byte, error) {
	ret := [][]byte{}
	err := fs.WalkDir(bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		lex, err := bundle.ReadFile(path)
		if err != nil {
			return err
		}
		ret = append(ret, lex)
		return nil
	})
	return ret, err
}

type SchemaFileWrapper struct {
	LexiconTypeID string `json:"$type,const=com.atproto.lexicon.schema" cborgen:"$type,const=com.atproto.lexicon.schema"`
	SchemaFile    lexicon.SchemaFile
}

func (sfw *SchemaFileWrapper) MarshalCBOR(w io.Writer) error {
	bs, err := json.Marshal(sfw.SchemaFile)
	if err != nil {
		return err
	}
	mapObj, err := data.UnmarshalJSON(bs)
	if err != nil {
		return err
	}
	mapObj["$type"] = "com.atproto.lexicon.schema"
	cbs, err := data.MarshalCBOR(mapObj)
	if err != nil {
		return err
	}
	_, err = w.Write(cbs)
	if err != nil {
		return err
	}
	return nil
}

func (sfw *SchemaFileWrapper) MarshalJSON() ([]byte, error) {
	bs, err := json.Marshal(sfw.SchemaFile)
	if err != nil {
		return nil, err
	}
	mapObj, err := data.UnmarshalJSON(bs)
	if err != nil {
		return nil, err
	}
	mapObj["$type"] = "com.atproto.lexicon.schema"
	bs, err = json.Marshal(mapObj)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

type SPKeyManager struct {
	priv *did.PrivKey
}

func (km *SPKeyManager) VerifyUserSignature(ctx context.Context, did string, sb []byte, sig []byte) error {
	panic("NYI")
}

func (km *SPKeyManager) SignForUser(ctx context.Context, did string, sb []byte) ([]byte, error) {
	return km.priv.Sign(sb)
}

func MakeLexiconRepo(ctx context.Context, cli *config.CLI) error {

	// CarStore = &carstore.SQLiteStore{}
	// err := CarStore.Open(cli.DataFilePath([]string{"carstore.db"}))

	// This just makes an empty file in "carstore" to verify the directory exists
	fd, err := cli.DataFileCreate([]string{"carstore", "empty"}, true)
	if err != nil {
		return err
	}
	sqlitePath := cli.DataFilePath([]string{"carstore", "meta.sqlite"})

    db, err := gorm.Open(sqlite.Open(sqlitePath))
	if err != nil {
		return err
	}
	err = fd.Close()
	if err != nil {
		return err
	}
	CarStore, err = carstore.NewCarStore(db, []string{
		cli.DataFilePath([]string{"carstore"}),
	})
	if err != nil {
		return err
	}

	// err := CarStore.Open(":memory:")
	// if err != nil {
	// 	return fmt.Errorf("failed to create carstore: %w", err)
	// }

	var priv *atcrypto.PrivateKeyK256
	exists, err := cli.DataFileExists([]string{"carstore", "repo.key"})
	if err != nil {
		return err
	}
	if exists {
		buf := bytes.Buffer{}
		err := cli.DataFileRead([]string{"carstore", "repo.key"}, &buf)
		if err != nil {
			return err
		}
		priv, err = atcrypto.ParsePrivateBytesK256(buf.Bytes())
		if err != nil {
			return err
		}
	} else {
		priv, err = atcrypto.GeneratePrivateKeyK256()
		if err != nil {
			return err
		}
		bs := priv.Bytes()
		err = cli.DataFileWrite([]string{"carstore", "repo.key"}, bytes.NewReader(bs), true)
		if err != nil {
			return err
		}
	}

	pub, err := priv.PublicKey()
	if err != nil {
		return fmt.Errorf("failed to get public key from private key: %w", err)
	}

	LexiconPubMultibase = pub.Multibase()
	signer := func(ctx context.Context, did string, sb []byte) ([]byte, error) {
		return priv.HashAndSign(sb)
	}

	ses, err := CarStore.NewDeltaSession(ctx, RepoUser, nil)
	if err != nil {
		return fmt.Errorf("failed to create delta session: %w", err)
	}

	root, err := CarStore.GetUserRepoHead(ctx, RepoUser)
	if err != nil {
		return fmt.Errorf("failed to get user repo head: %w", err)
	}

	LexiconRepo, err = atrepo.OpenRepo(ctx, ses, root)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	// LexiconRepo = atrepo.NewRepo(ctx, cli.MyDID(), ses)

	LexiconPubMultibase = pub.Multibase()
	lexs, err := walkLexicons(ctx, lexicons.AllFiles, "/")
	if err != nil {
		return fmt.Errorf("failed to walk lexicon files: %w", err)
	}
	for _, lex := range lexs {
		lexFile := lexicon.SchemaFile{}
		err := json.Unmarshal(lex, &lexFile)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(lexFile.ID, "place.stream") {
			continue
		}
		sfw := &SchemaFileWrapper{SchemaFile: lexFile}
		rpath := fmt.Sprintf("com.atproto.lexicon.schema/%s", lexFile.ID)
		newCid, err := GetCID(sfw)
		if err != nil {
			return err
		}

		oldCid, _, err := LexiconRepo.GetRecord(ctx, rpath)
		if errors.Is(err, mst.ErrNotFound) {
			_, err = LexiconRepo.PutRecord(ctx, rpath, sfw)
			if err != nil {
				return err
			}
			log.Log(ctx, "created new lexicon record", "rpath", rpath, "cid", newCid.String())
		} else if err != nil {
			return err
		} else {
			if newCid.Equals(oldCid) {
				log.Log(ctx, "new cid is the same as old cid, skipping lexicon record", "rpath", rpath, "cid", newCid.String())
				continue
			} else {
				log.Log(ctx, "new cid is different from old cid, updating lexicon record", "rpath", rpath, "old", oldCid.String(), "new", newCid.String())
				_, err = LexiconRepo.UpdateRecord(ctx, rpath, sfw)
				if err != nil {
					return err
				}
			}
		}
	}
	c, rev, err := LexiconRepo.Commit(ctx, signer)
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	log.Log(ctx, "LexiconRepo committed", "cid", c.String(), "rev", rev)
	_, err = ses.CloseWithRoot(ctx, c, rev)
	if err != nil {
		return fmt.Errorf("failed to close delta session: %w", err)
	}
	roses, err := CarStore.NewDeltaSession(ctx, RepoUser, &rev)
	if err != nil {
		return fmt.Errorf("handleComAtprotoRepoListRecords: failed to create delta session: %w", err)
	}

	base := roses.BaseCid()
	if base == cid.Undef {
		return   fmt.Errorf("handleComAtprotoRepoListRecords: delta session has no base cid")
	}
	return nil
}

func OpenLexiconRepo(ctx context.Context, cli *config.CLI) (*atrepo.Repo, *carstore.DeltaSession, error) {
	ses, err := CarStore.NewDeltaSession(ctx, RepoUser, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("handleComAtprotoRepoListRecords: failed to create delta session: %w", err)
	}

	base := ses.BaseCid()
	if base == cid.Undef {
		return   nil, nil, fmt.Errorf("handleComAtprotoRepoListRecords: delta session has no base cid")
	}

	r, err := atrepo.OpenRepo(ctx, ses, base)
	if err != nil {
		return nil, nil, fmt.Errorf("handleComAtprotoRepoListRecords: failed to open repo: %w", err)
	}
	return r, ses, nil
}

// Get record that handles special-casing for com.atproto.lexicon.schema
func GetRecordCBOR(ctx context.Context, ses *carstore.DeltaSession, c cid.Cid, collection string, rkey string) (cbg.CBORMarshaler, error) {
	b, err := ses.Get(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("handleComAtprotoRepoListRecords: failed to get record for collection %q, rkey %q: %w", collection, rkey, err)
	}
	var val cbg.CBORMarshaler
	if collection == "com.atproto.lexicon.schema" {
		sfMap, err := data.UnmarshalCBOR(b.RawData())
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema file: %w", err)
		}
		jbs, err := json.Marshal(sfMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema file: %w", err)
		}
		sf := lexicon.SchemaFile{}
		err = json.Unmarshal(jbs, &sf)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema file: %w", err)
		}
		val = &SchemaFileWrapper{
			SchemaFile: sf,
		}
	} else {
		val, err = lexutil.CborDecodeValue(b.RawData())
		if err != nil {
			return nil, fmt.Errorf("failed to decode record: %w", err)
		}
	}
	return val, nil
}