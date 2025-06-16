package atproto

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	atrepo "github.com/bluesky-social/indigo/atproto/repo"
	"github.com/bluesky-social/indigo/repo"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/config"
)

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

func MakeLexiconRepo(ctx context.Context, cli *config.CLI) error {
	// catalog := lexicon.NewBaseCatalog()
	// err := catalog.LoadEmbedFS(lexicons.AllFiles)
	// if err != nil {
	// 	return err
	// }
	bs := atrepo.NewTinyBlockstore()
	did := fmt.Sprintf("did:web:%s", cli.PublicHost)
	_ = repo.NewRepo(ctx, did, bs)
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
	}
	panic("done")
	return nil
}
