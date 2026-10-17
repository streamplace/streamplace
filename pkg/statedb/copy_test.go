package statedb

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// A sqlite → sqlite copy exercises everything but the Postgres-only sequence
// bump: batching, soft-deleted rows, JSON normalization, idempotency, and
// the per-table report.
func TestCopyState(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fromURL := "sqlite://" + filepath.Join(dir, "from.sqlite")
	toURL := "sqlite://" + filepath.Join(dir, "to.sqlite")

	src, err := MakeDB(ctx, &config.CLI{DBURL: fromURL}, nil, nil)
	require.NoError(t, err)
	require.NoError(t, src.DB.Create(&[]AppTask{
		{Type: "a", Status: "PENDING", Payload: json.RawMessage(`{"n":1}`)},
		{Type: "b", Status: "PENDING", Payload: json.RawMessage("")}, // "" would be rejected by jsonb
		{Type: "c", Status: "COMPLETED", Payload: json.RawMessage(`{"n":3}`)},
	}).Error)
	for i := 0; i < 1203; i++ { // more than two batches
		require.NoError(t, src.DB.Create(&Repo{DID: fmt.Sprintf("did:plc:%04d", i), IndexedAt: time.Now()}).Error)
	}
	require.NoError(t, src.DB.Create(&BrandingBlob{BroadcasterID: "did:web:x", Key: "networkName", MimeType: "text/plain", Data: []byte("W")}).Error)
	gone := BrandingBlob{BroadcasterID: "did:web:x", Key: "oldKey", MimeType: "text/plain", Data: []byte("bye")}
	require.NoError(t, src.DB.Create(&gone).Error)
	require.NoError(t, src.DB.Delete(&gone).Error) // soft
	require.NoError(t, src.DB.Create(&Config{Key: "k", Value: []byte{0, 1, 2, 255}}).Error)

	reports, err := CopyState(ctx, fromURL, toURL, 500)
	require.NoError(t, err)
	byTable := map[string]CopyReport{}
	for _, r := range reports {
		byTable[r.Table] = r
	}
	require.Equal(t, int64(3), byTable["app_tasks"].Inserted)
	require.Equal(t, int64(1203), byTable["repos"].Inserted)
	require.Equal(t, int64(2), byTable["branding_blobs"].Inserted, "the soft-deleted row travels too")
	require.Equal(t, int64(1), byTable["configs"].Inserted)

	dst, err := MakeDB(ctx, &config.CLI{DBURL: toURL}, nil, nil)
	require.NoError(t, err)
	var deleted int64
	require.NoError(t, dst.DB.Unscoped().Model(&BrandingBlob{}).Where("deleted_at IS NOT NULL").Count(&deleted).Error)
	require.Equal(t, int64(1), deleted)
	var b AppTask
	require.NoError(t, dst.DB.Where("type = ?", "b").First(&b).Error)
	require.Empty(t, b.Payload, "empty payload becomes NULL")
	var c Config
	require.NoError(t, dst.DB.First(&c, "key = ?", "k").Error)
	require.Equal(t, []byte{0, 1, 2, 255}, c.Value)

	// Again, after the source moved on (the delta pass after a cutover):
	// nothing duplicated, and rows that changed under their existing keys
	// are carried over rather than left at their first-pass values.
	require.NoError(t, src.DB.Model(&Config{}).Where("key = ?", "k").Update("value", []byte{9}).Error)
	require.NoError(t, src.DB.Model(&AppTask{}).Where("type = ?", "a").Update("status", "COMPLETED").Error)
	reports, err = CopyState(ctx, fromURL, toURL, 100)
	require.NoError(t, err)
	for _, r := range reports {
		require.Equal(t, r.Source, r.Target, r.Table)
	}
	require.NoError(t, dst.DB.First(&c, "key = ?", "k").Error)
	require.Equal(t, []byte{9}, c.Value, "the changed config value came across")
	var a AppTask
	require.NoError(t, dst.DB.Where("type = ?", "a").First(&a).Error)
	require.Equal(t, "COMPLETED", string(a.Status), "the changed task status came across")
}

func TestNormalizeJSON(t *testing.T) {
	rows := []AppTask{
		{Payload: json.RawMessage(`{"ok":true}`)},
		{Payload: json.RawMessage("")},
		{Payload: json.RawMessage("not json")},
		{Payload: nil},
	}
	normalizeJSON(reflect.ValueOf(rows))
	require.JSONEq(t, `{"ok":true}`, string(rows[0].Payload))
	require.Nil(t, rows[1].Payload)
	require.Nil(t, rows[2].Payload)
	require.Nil(t, rows[3].Payload)
}

func TestDialectorFor(t *testing.T) {
	_, typ, err := dialectorFor("sqlite:///tmp/x.sqlite")
	require.NoError(t, err)
	require.Equal(t, DBTypeSQLite, typ)
	_, typ, err = dialectorFor("postgresql://u:p@h/db")
	require.NoError(t, err)
	require.Equal(t, DBTypePostgres, typ)
	_, _, err = dialectorFor("mysql://nope")
	require.Error(t, err)
}
