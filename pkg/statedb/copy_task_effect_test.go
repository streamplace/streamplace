package statedb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestCopyStateTaskEffects(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		name := "with receipts"
		if legacy {
			name = "before receipts"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			dir := t.TempDir()
			fromURL := "sqlite://" + filepath.Join(dir, "from.sqlite")
			dstConfig := &config.CLI{DBURL: "sqlite://" + filepath.Join(dir, "to.sqlite")}
			src, err := MakeDB(ctx, &config.CLI{DBURL: fromURL}, nil, nil)
			require.NoError(t, err)
			finished, err := src.EnqueueTask(ctx, "finished", nil)
			require.NoError(t, err)
			claimed := dequeueTestTask(t, src, finished.Type)
			require.NoError(t, src.CompleteTask(ctx, claimed.ID, claimed.ClaimToken))

			var blockedID uint
			var want []AppTaskEffect
			if legacy {
				require.NoError(t, src.DB.Migrator().DropTable(&AppTaskEffect{}))
			} else {
				blocked := enqueueClaimedTask(t, src, "publication", nil)
				blockedID = blocked.ID
				const target = "at://did:web:creator.example/place.stream.video/video-rkey"
				want = []AppTaskEffect{
					{TaskID: blockedID, EffectKey: "track", Receipt: []byte(`{"cid":"confirmed"}`)},
					{TaskID: blockedID, EffectKey: "video", Target: target},
				}
				for _, effect := range want {
					_, err := src.beginTaskEffect(ctx, blocked.ID, blocked.ClaimToken, effect.EffectKey, effect.Target)
					require.NoError(t, err)
					if effect.Receipt != nil {
						require.NoError(t, src.setTaskEffect(ctx, blocked.ID, blocked.ClaimToken, effect.EffectKey, effect.Receipt, time.Time{}))
					}
				}
				require.ErrorIs(t, src.CompleteTask(ctx, blocked.ID, blocked.ClaimToken), ErrTaskEffectUncertain)
			}

			// With batch=1, the copy cursor must include EffectKey as well as TaskID.
			_, err = CopyState(ctx, fromURL, dstConfig, 1)
			require.NoError(t, err)
			dst, err := MakeDB(ctx, dstConfig, nil, nil)
			require.NoError(t, err)
			requireTaskStatus(t, dst, finished.ID, TaskStatusCompleted)
			if !legacy {
				requireTaskStatus(t, dst, blockedID, TaskStatusBlocked)
			}
			var got []AppTaskEffect
			require.NoError(t, dst.DB.Order("task_id, effect_key").Find(&got).Error)
			require.ElementsMatch(t, want, got)

			if legacy {
				require.NoError(t, src.DB.Migrator().DropTable(&Repo{}))
				_, err := CopyState(ctx, fromURL, dstConfig, 1)
				require.ErrorContains(t, err, "repos: counting source rows")
			}
		})
	}
}
