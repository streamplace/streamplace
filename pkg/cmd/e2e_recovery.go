package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
)

// seedE2ERecovery reproduces an old node's persisted index and server repo.
// These are listing fixtures only: no media payload is written or promised.
func seedE2ERecovery(ctx context.Context, dataDir, broadcasterHost, accountDID string, client *xrpc.Client) (err error) {
	// Match the fork's defaults, including ServerHost's broadcaster fallback
	// and the host-scoped server signing key stored in state.sqlite.
	cli := config.CLI{
		DataDir:         dataDir,
		BroadcasterHost: broadcasterHost,
		ServerHost:      broadcasterHost,
		DBURL:           "sqlite://" + dataDir + "/state.sqlite",
	}
	mod, err := model.MakeDB(cli.DataFilePath([]string{"index"}))
	if err != nil {
		return fmt.Errorf("open recovery index: %w", err)
	}
	indexDB, err := mod.(*model.DBModel).DB.DB()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, indexDB.Close()) }()
	state, err := statedb.MakeDB(ctx, &cli, nil, mod)
	if err != nil {
		return fmt.Errorf("open recovery state: %w", err)
	}
	stateDB, err := state.DB.DB()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stateDB.Close()) }()
	serverRepo, err := atproto.MakeServerRepo(ctx, &cli, state)
	if err != nil {
		return fmt.Errorf("open recovery server repo: %w", err)
	}
	// All SQLite handles are closed on return, before nodeCmd.Start opens
	// the same files. The temporary data directory remains harness-owned.
	defer func() { err = errors.Join(err, serverRepo.Close()) }()

	for _, title := range []string{"e2e recovered video", "e2e unhosted video"} {
		blobCID := bdasl.CID([]byte(title))
		track := placestream.MediaTrack{
			LexiconTypeID: constants.PLACE_STREAM_MEDIA_TRACK,
			Track: placestream.MediaTrack_Track{
				MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{
					LexiconTypeID: "place.stream.media.defs#muxlTrack",
					Blob:          blobCID,
					TrackId:       "1",
					MediaType:     "video",
				},
			},
		}
		// Publish the user's records too, so repo sweeps cannot delete the
		// preindexed fixtures as records absent from the authoritative PDS.
		trackURI, err := createRecord(ctx, client, constants.PLACE_STREAM_MEDIA_TRACK, accountDID, &track)
		if err != nil {
			return fmt.Errorf("publish recovery track: %w", err)
		}
		trackATURI, err := syntax.ParseATURI(trackURI)
		if err != nil {
			return err
		}
		if err := mod.UpsertMediaTrack(ctx, track, trackATURI); err != nil {
			return err
		}
		trackCID, err := spid.GetCID(&track)
		if err != nil {
			return err
		}
		video := placestream.Video{
			LexiconTypeID: constants.PLACE_STREAM_VIDEO,
			CreatedAt:     time.Now().UTC().Format(util.ISO8601),
			Title:         title,
			DurationMs:    10_000,
			Source: placestream.Video_Source{
				MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
					LexiconTypeID: "place.stream.media.defs#sourceTracks",
					Tracks: []comatproto.RepoStrongRef{
						{LexiconTypeID: "com.atproto.repo.strongRef", Uri: trackURI, Cid: trackCID.String()},
					},
				},
			},
		}
		videoURI, err := createRecord(ctx, client, constants.PLACE_STREAM_VIDEO, accountDID, &video)
		if err != nil {
			return fmt.Errorf("publish recovery video: %w", err)
		}
		videoATURI, err := syntax.ParseATURI(videoURI)
		if err != nil {
			return err
		}
		if err := mod.UpsertVideo(ctx, video, videoATURI); err != nil {
			return err
		}
		if title == "e2e recovered video" {
			// Deliberately do not index this origin. Startup must recover it
			// from the node-owned repo, not an admin request or firehose.
			if err := atproto.CommitServerRepoRecord(ctx, &cli, constants.PLACE_STREAM_MEDIA_ORIGIN, blobCID, &placestream.MediaOrigin{
				LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
				Blob:          blobCID,
				Size:          int64(len(title)),
				MimeType:      "video/mp4",
			}); err != nil {
				return fmt.Errorf("commit recovery origin: %w", err)
			}
		}
	}
	// Age replay events past the node's 72-hour retention cutoff. The
	// durable repo still owns the origin, but boot prunes its event so a
	// self-subscription cannot mask a missing startup reconciliation.
	commits, err := gorm.Open(sqlite.Open(cli.DataFilePath([]string{"server-repo", "commits.db"})), &gorm.Config{Logger: config.GormLogger})
	if err != nil {
		return fmt.Errorf("open recovery commit events: %w", err)
	}
	commitDB, err := commits.DB()
	if err != nil {
		return fmt.Errorf("get recovery commit database: %w", err)
	}
	defer func() { err = errors.Join(err, commitDB.Close()) }()
	if err := commits.WithContext(ctx).Model(&atproto.ServerCommitEvent{}).
		Where("repo_did = ?", cli.ServerDID()).
		Update("timestamp", time.Now().Add(-96*time.Hour)).Error; err != nil {
		return fmt.Errorf("age recovery commit events: %w", err)
	}
	return nil
}
