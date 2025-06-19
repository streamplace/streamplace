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
	atrepo "github.com/bluesky-social/indigo/atproto/repo"
	"github.com/bluesky-social/indigo/repo"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/config"
)

var LexiconRepo *repo.Repo
var LexiconPubMultibase string

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
	schemaFile lexicon.SchemaFile
}

func (sfw *SchemaFileWrapper) MarshalCBOR(w io.Writer) error {
	bs, err := json.Marshal(sfw.schemaFile)
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

type SPKeyManager struct {
	priv atcrypto.PrivateKey
}

func (km *SPKeyManager) VerifyUserSignature(ctx context.Context, did string, sb []byte, sig []byte) error {
	panic("NYI")
}

func (km *SPKeyManager) SignForUser(ctx context.Context, did string, sb []byte) ([]byte, error) {
	return km.priv.HashAndSign(sb)
}

func MakeLexiconRepo(ctx context.Context, cli *config.CLI) error {
	// db, err := gorm.Open(sqlite.Open(":memory:"))
	// if err != nil {
	// 	return err
	// }
	// cs, err := carstore.NewNonArchivalCarstore(db)
	// if err != nil {
	// 	return err
	// }
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		return err
	}

	// km := &SPKeyManager{
	// 	priv: priv,
	// }

	// repoman := repomgr.NewRepoManager(cs, km)

	// if err := repoman.InitNewActor(ctx, models.Uid(0), cli.PublicHost, fmt.Sprintf("did:web:%s", cli.PublicHost), "", "", ""); err != nil {
	// 	return err
	// }

	pub, err := priv.PublicKey()
	if err != nil {
		return err
	}
	LexiconPubMultibase = pub.Multibase()
	signer := func(ctx context.Context, did string, sb []byte) ([]byte, error) {
		return priv.HashAndSign(sb)
	}
	// catalog := lexicon.NewBaseCatalog()
	// err := catalog.LoadEmbedFS(lexicons.AllFiles)
	// if err != nil {
	// 	return err
	// }
	bs := atrepo.NewTinyBlockstore()
	did := fmt.Sprintf("did:web:%s", cli.PublicHost)
	LexiconRepo = repo.NewRepo(ctx, did, bs)
	lexs, err := walkLexicons(ctx, lexicons.AllFiles, "/")
	if err != nil {
		return err
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
		sfw := &SchemaFileWrapper{schemaFile: lexFile}
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
	_, _, err = LexiconRepo.Commit(context.TODO(), signer)
	if err != nil {
		return err
	}
	return nil
}
