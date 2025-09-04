package atproto

import (
	"context"
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

func TestLexiconRepo(t *testing.T) {
	cli := config.CLI{
		PublicHost: "example.com",
		DBURL:      ":memory:",
	}
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(&cli, nil, mod)
	require.NoError(t, err)

	// creating a new repo
	handle, err := MakeLexiconRepo(context.Background(), &cli, mod, state)
	require.NoError(t, err)
	r, sess, err := OpenLexiconRepo(context.Background())
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NotNil(t, sess)
	c, _, err := r.GetRecord(context.Background(), "com.atproto.lexicon.schema/place.stream.chat.message")
	require.NoError(t, err)
	rec, err := GetRecordCBOR(context.Background(), sess, c, "com.atproto.lexicon.schema", "place.stream.chat.message")
	require.NoError(t, err)
	require.NotNil(t, rec)
	handle.Close()

	evts, err := state.GetCommitEventsSinceSeq(cli.MyDID(), 0)
	require.NoError(t, err)
	require.Len(t, evts, 1)
	require.Equal(t, evts[0].RepoDID, cli.MyDID())

	// opening an existing repo
	handle, err = MakeLexiconRepo(context.Background(), &cli, mod, state)
	require.NoError(t, err)
	handle.Close()

	// Walk all files and build a map of file contents, modifying the first file
	files := map[string]*fstest.MapFile{}

	err = fs.WalkDir(lexicons.AllFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		contents, err := lexicons.AllFiles.ReadFile(path)
		if err != nil {
			return err
		}
		if path == "place/stream/chat/message.json" {
			// Modify the first file encountered (for example, append a newline)
			data := map[string]any{}
			err := json.Unmarshal(contents, &data)
			if err != nil {
				return err
			}
			data["defs"].(map[string]any)["main"].(map[string]any)["description"] = "Some kind of chat nonsense."
			contents, err = json.Marshal(data)
			if err != nil {
				return err
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files[path] = &fstest.MapFile{
			Data:    contents,
			Mode:    0444,
			ModTime: info.ModTime(),
		}
		return nil
	})
	require.NoError(t, err)
	modifiedFS := fstest.MapFS{}
	for k, v := range files {
		modifiedFS[k] = v
	}
	AllFiles = modifiedFS

	// opening an existing repo with modified lexicon
	handle, err = MakeLexiconRepo(context.Background(), &cli, mod, state)
	require.NoError(t, err)
	handle.Close()

	evts, err = state.GetCommitEventsSinceSeq(cli.MyDID(), 0)
	require.NoError(t, err)
	require.Len(t, evts, 2)
	require.Equal(t, evts[0].RepoDID, cli.MyDID())
	require.Equal(t, evts[1].RepoDID, cli.MyDID())
	oldCommit, err := evts[0].ToCommitEvent()
	require.NoError(t, err)
	newCommit, err := evts[1].ToCommitEvent()
	require.NoError(t, err)
	require.Equal(t, newCommit.Since, &oldCommit.Rev)
	require.Equal(t, newCommit.PrevData.String(), evts[0].SignedData)
}
