package director

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/redcircle"
)

func downloadImage(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, errors.New("empty URL provided")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := aqhttp.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	return imageData, nil
}

func (ss *StreamSession) UpdateProfilePicture(ctx context.Context, repoDID string) error {
	session, err := ss.statefulDB.GetSessionByDID(ss.repoDID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("no session found for repoDID: %s", ss.repoDID)
	}
	session, err = ss.op.RefreshIfNeeded(session)
	if err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}
	client, err := ss.op.GetXrpcClient(session)
	if err != nil {
		return fmt.Errorf("failed to get xrpc client: %w", err)
	}
	profile, err := ss.atsync.FetchUserProfile(ctx, repoDID)
	if err != nil {
		return fmt.Errorf("failed to fetch user profile: %w", err)
	}
	if profile == nil {
		return fmt.Errorf("received nil profile from Bluesky API for '%s'", repoDID)
	}
	if profile.Avatar == nil || *profile.Avatar == "" {
		return fmt.Errorf("received nil or empty avatar from Bluesky API for '%s'", repoDID)
	}
	profileAvatar := *profile.Avatar
	imageData, err := downloadImage(ctx, profileAvatar)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	circleImage, err := redcircle.GenerateRedCircle(ctx, imageData)
	if err != nil {
		return fmt.Errorf("failed to generate redcircle: %w", err)
	}
	var out comatproto.RepoUploadBlob_Output
	err = client.Do(ctx, xrpc.Procedure, "image/jpeg", "com.atproto.repo.uploadBlob", nil, bytes.NewReader(circleImage), &out)
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", err)
	}
	if out.Blob.Size == 0 {
		return fmt.Errorf("uploaded blob has size 0")
	}
	var profileRecord comatproto.RepoGetRecord_Output
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       repoDID,
		"collection": "app.bsky.actor.profile",
		"rkey":       "self",
	}, nil, &profileRecord)
	if err != nil {
		return fmt.Errorf("failed to get profile record: %w", err)
	}
	actorProfile, ok := profileRecord.Value.Val.(*bsky.ActorProfile)
	if !ok {
		return fmt.Errorf("profile record is not a actor profile")
	}
	actorProfile.Avatar = out.Blob
	inp := comatproto.RepoPutRecord_Input{
		Collection: "app.bsky.actor.profile",
		Record:     &lexutil.LexiconTypeDecoder{Val: actorProfile},
		Rkey:       "self",
		Repo:       repoDID,
		SwapRecord: profileRecord.Cid,
	}
	putOutput := comatproto.RepoPutRecord_Output{}
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &putOutput)
	if err != nil {
		return fmt.Errorf("failed to put profile record: %w", err)
	}
	log.Log(ctx, "uploaded redcircle", "ref", out.Blob.Ref.String(), "size", out.Blob.Size)
	return nil
}
