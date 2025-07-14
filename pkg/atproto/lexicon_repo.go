package atproto

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strings"

	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/bluesky-social/indigo/carstore"
	"github.com/bluesky-social/indigo/models"
	atrepo "github.com/bluesky-social/indigo/repo"
	"github.com/ipfs/go-cid"
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

    db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		return err
	}
	fd, err := cli.DataFileCreate([]string{"carstore", "example"}, true)
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
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		return err
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

	LexiconRepo = atrepo.NewRepo(ctx, cli.MyDID(), ses)

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
		_, err = LexiconRepo.PutRecord(context.TODO(), rpath, sfw)
		if err != nil {
			return err
		}
		_, _, err = LexiconRepo.GetRecord(context.TODO(), rpath)
		if err != nil {
			return fmt.Errorf("failed to get record %s: %w", rpath, err)
		} else {
			fmt.Printf("put record %s\n", rpath)
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
