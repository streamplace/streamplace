package branding

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

const bid = "did:web:node.example"

func newState(t *testing.T) *statedb.StatefulDB {
	t.Helper()
	cli := &config.CLI{BroadcasterHost: "node.example", DBURL: ":memory:"}
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), cli, nil, mod)
	require.NoError(t, err)
	return state
}

func zipFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func readZip(t *testing.T, bs []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(bs), int64(len(bs)))
	require.NoError(t, err)
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		out[f.Name] = data
	}
	return out
}

func TestExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newState(t)
	require.NoError(t, src.PutBrandingBlob(bid, "siteTitle", TextMime, []byte("Sample"), nil, nil))
	require.NoError(t, src.PutBrandingBlob(bid, "primaryColor", TextMime, []byte("#11e8b2"), nil, nil))
	// stored with spaces and unsorted keys, as the admin UI would write it
	require.NoError(t, src.PutBrandingBlob(bid, "navLinks", TextMime, []byte(`[{"url": "https://example.com", "label": "Home", "icon": "home"}]`), nil, nil))
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="1"/></svg>`)
	require.NoError(t, src.PutBrandingBlob(bid, "mainLogo", "image/svg+xml", svg, nil, nil))

	bs, err := Export(ctx, src, bid)
	require.NoError(t, err)
	files := readZip(t, bs)
	require.Contains(t, files, "branding.yaml")
	require.Equal(t, svg, files["mainLogo.svg"])
	y := string(files["branding.yaml"])
	require.Contains(t, y, "siteTitle: Sample")
	require.Contains(t, y, `primaryColor: '#11e8b2'`)
	require.Contains(t, y, "mainLogo: mainLogo.svg")
	require.Contains(t, y, "label: Home", "JSON keys export as YAML structures")
	require.Contains(t, y, "defaults:")

	dst := newState(t)
	require.NoError(t, dst.PutBrandingBlob(bid, "accentColor", TextMime, []byte("#ff0000"), nil, nil))
	report, err := Import(ctx, dst, bid, bs, false, true)
	require.NoError(t, err)
	require.False(t, report.Applied, "dry run")
	actions := map[string]string{}
	for _, c := range report.Changes {
		actions[c.Key] = c.Action
	}
	require.Equal(t, "added", actions["siteTitle"])
	require.Equal(t, "added", actions["mainLogo"])
	require.Equal(t, "removed", actions["accentColor"], "replace mode removes keys the bundle lacks")
	keys, _ := dst.ListBrandingKeys(bid)
	require.Equal(t, []string{"accentColor"}, keys, "dry run wrote nothing")

	report, err = Import(ctx, dst, bid, bs, false, false)
	require.NoError(t, err)
	require.True(t, report.Applied)
	got, err := dst.GetBrandingBlob(bid, "mainLogo")
	require.NoError(t, err)
	require.Equal(t, svg, got.Data)
	require.Equal(t, "image/svg+xml", got.MimeType)
	nav, err := dst.GetBrandingBlob(bid, "navLinks")
	require.NoError(t, err)
	require.JSONEq(t, `[{"label":"Home","url":"https://example.com","icon":"home"}]`, string(nav.Data))
	_, err = dst.GetBrandingBlob(bid, "accentColor")
	require.Error(t, err, "removed")

	// A second import of the same bundle is a no-op.
	report, err = Import(ctx, dst, bid, bs, false, true)
	require.NoError(t, err)
	for _, c := range report.Changes {
		require.Equal(t, "unchanged", c.Action, c.Key)
	}
}

func TestImportMergeKeepsUnmentionedKeys(t *testing.T) {
	ctx := context.Background()
	dst := newState(t)
	require.NoError(t, dst.PutBrandingBlob(bid, "accentColor", TextMime, []byte("#ff0000"), nil, nil))
	bs := zipFiles(t, map[string][]byte{"branding.yaml": []byte("version: 1\nbranding:\n  siteTitle: Merged\n")})
	report, err := Import(ctx, dst, bid, bs, true, false)
	require.NoError(t, err)
	require.True(t, report.Applied)
	_, err = dst.GetBrandingBlob(bid, "accentColor")
	require.NoError(t, err, "merge keeps it")
	title, err := dst.GetBrandingBlob(bid, "siteTitle")
	require.NoError(t, err)
	require.Equal(t, "Merged", string(title.Data))
}

func TestImportValidation(t *testing.T) {
	ctx := context.Background()
	dst := newState(t)
	cases := map[string]string{
		"bad color":        "branding:\n  primaryColor: green\n",
		"bad enum":         "branding:\n  streamLayout: fancy\n",
		"missing file":     "branding:\n  mainLogo: nope.svg\n",
		"future version":   "version: 99\nbranding: {}\n",
		"json not a value": "branding:\n  navCta: '{not json'\n",
	}
	for name, y := range cases {
		bs := zipFiles(t, map[string][]byte{"branding.yaml": []byte(y)})
		_, err := Import(ctx, dst, bid, bs, false, true)
		require.Error(t, err, name)
	}
	_, err := Import(ctx, dst, bid, []byte("not a zip"), false, true)
	require.Error(t, err)
	_, err = Import(ctx, dst, bid, zipFiles(t, map[string][]byte{"other.txt": []byte("x")}), false, true)
	require.Error(t, err, "no branding.yaml")

	// unknown keys warn but import as text; nested folders are fine
	bs := zipFiles(t, map[string][]byte{
		"theme/branding.yaml": []byte("branding:\n  futureKey: hello\n  favicon: theme/icon.png\n"),
		"theme/icon.png":      []byte("\x89PNG"),
	})
	report, err := Import(ctx, dst, bid, bs, false, true)
	require.NoError(t, err)
	require.NotEmpty(t, report.Warnings)
}

func TestNormalize(t *testing.T) {
	v, err := Normalize("primaryColor", []byte("  #ABCDEF "))
	require.NoError(t, err)
	require.Equal(t, "#abcdef", string(v))
	_, err = Normalize("primaryColor", []byte("rgb(1,2,3)"))
	require.Error(t, err)
	v, err = Normalize("typeface", []byte("inter"))
	require.NoError(t, err)
	require.Equal(t, "inter", string(v))
	_, err = Normalize("typeface", []byte("comic"))
	require.Error(t, err)
	v, err = Normalize("siteTitle", []byte(""))
	require.NoError(t, err)
	require.Empty(t, v)
	v, err = Normalize("unknownKey", []byte("anything"))
	require.NoError(t, err)
	require.Equal(t, "anything", string(v))
}
