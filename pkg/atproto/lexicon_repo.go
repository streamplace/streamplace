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
	"github.com/bluesky-social/indigo/repomgr"

	"github.com/whyrusleeping/go-did"
	secpEc "gitlab.com/yawning/secp256k1-voi/secec"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/config"
)

// var LexiconRepo *repo.Repo
var LexiconRepo *repomgr.RepoManager
var LexiconPubMultibase string
var RepoUser models.Uid = models.Uid(1)

func init() {
	err := MakeLexiconRepo(context.Background(), &config.CLI{
		PublicHost: "fairway.iameli.link",
	})
	if err != nil {
		panic(err)
	}
}

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
	cs := &carstore.SQLiteStore{}
	err := cs.Open("/home/iameli/carstore.db")
	if err != nil {
		return fmt.Errorf("failed to create carstore: %w", err)
	}
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	k, err := secpEc.NewPrivateKey(priv.Bytes())
	if err != nil {
		return fmt.Errorf("failed to create secp256k1 private key: %w", err)
	}
	serkey := &did.PrivKey{
		Raw:  k,
		Type: did.KeyTypeSecp256k1,
	}

	km := &SPKeyManager{
		priv: serkey,
	}

	repoman := repomgr.NewRepoManager(cs, km)

	if err := repoman.InitNewActor(ctx, RepoUser, cli.PublicHost, fmt.Sprintf("did:web:%s", cli.PublicHost), "foo", "", ""); err != nil {
		return fmt.Errorf("failed to initialize new actor: %w", err)
	}
	pub, err := priv.PublicKey()
	if err != nil {
		return fmt.Errorf("failed to get public key from private key: %w", err)
	}
	LexiconPubMultibase = pub.Multibase()
	lexs, err := walkLexicons(ctx, lexicons.AllFiles, "/")
	if err != nil {
		return fmt.Errorf("failed to walk lexicon files: %w", err)
	}
	for _, lex := range lexs {
		lexFile := lexicon.SchemaFile{}
		err := json.Unmarshal(lex, &lexFile)
		if err != nil {
			return fmt.Errorf("failed to unmarshal lexicon file: %w", err)
		}
		if !strings.HasPrefix(lexFile.ID, "place.stream") {
			continue
		}
		sfw := &SchemaFileWrapper{SchemaFile: lexFile}
		rpath := fmt.Sprintf("com.atproto.lexicon.schema/%s", lexFile.ID)
		_, _, err = repoman.PutRecord(context.TODO(), RepoUser, "com.atproto.lexicon.schema", lexFile.ID, sfw)
		if err != nil {
			return fmt.Errorf("failed to put record %s: %w", rpath, err)
		}
		// _, _, err = repoman.GetRecord(context.TODO(), RepoUser, "com.atproto.lexicon.schema", rkey, cid.Cid{})
		// if err != nil {
		// 	return fmt.Errorf("failed to get record %s: %w", rpath, err)
		// } else {
		// 	fmt.Printf("put record %s\n", rpath)
		// }
	}
	LexiconRepo = repoman
	return nil
}
