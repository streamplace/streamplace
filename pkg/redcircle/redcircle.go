package redcircle

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atdata"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/streamplace/oatproxy/pkg/oatproxy"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

const WIDTH = 1000.0
const HEIGHT = 1000.0

const BORDER_SIZE = 62.0

const BorderScaleFactor = WIDTH / (WIDTH - BORDER_SIZE*2)

//go:embed redcircle.png
var RedCirclePNG []byte

func GenerateRedCircle(ctx context.Context, profileJPEG []byte) ([]byte, error) {
	c := canvas.New(WIDTH, HEIGHT)
	reader := bytes.NewReader(RedCirclePNG)
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	profileReader := bytes.NewReader(profileJPEG)
	profileImg, _, err := image.Decode(profileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	scaleFactor := (float64(profileImg.Bounds().Max.X) / float64(WIDTH)) * BorderScaleFactor

	// Draw the decoded image onto the canvas
	canvasCtx := canvas.NewContext(c)
	canvasCtx.DrawImage(BORDER_SIZE, BORDER_SIZE, profileImg, canvas.DPMM(scaleFactor))
	canvasCtx.DrawImage(0, 0, img, canvas.DPMM(1))

	var buf bytes.Buffer

	if err := c.Write(&buf, renderers.JPEG()); err != nil {
		return nil, fmt.Errorf("failed to render to JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

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

func UpdateProfilePicture(ctx context.Context, repoDID string, client *oatproxy.XrpcClient, mod model.Model) error {
	oldProfile, err := mod.GetBskyProfile(ctx, repoDID, false)
	if err != nil {
		return fmt.Errorf("failed to get old profile: %w", err)
	}
	if oldProfile == nil {
		return fmt.Errorf("no old profile found for repoDID: %s", repoDID)
	}
	if oldProfile.Avatar == nil {
		return fmt.Errorf("no avatar found for old profile")
	}

	repo, err := mod.GetRepo(repoDID)
	if err != nil {
		return fmt.Errorf("failed to get repo: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("no repo found for repoDID: %s", repoDID)
	}

	// https://iameli.com/xrpc/com.atproto.sync.getBlob?did=did:plc:2zmxikig2sj7gqaezl5gntae&cid=bafkreia2myy6nak4daop6dnny76ntilhel5wxpcejmvmwj5t3i3npcfifa
	avatarURL := fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s", repo.PDS, repoDID, oldProfile.Avatar.Ref.String())

	imageData, err := downloadImage(ctx, avatarURL)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	circleImage, err := GenerateRedCircle(ctx, imageData)
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
	var getRecordOut comatproto.RepoGetRecord_Output
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       repoDID,
		"collection": "app.bsky.actor.profile",
		"rkey":       "self",
	}, nil, &getRecordOut)
	if err != nil {
		return fmt.Errorf("failed to get profile record: %w", err)
	}
	oldAvatar := oldProfile.Avatar
	oldProfile.Avatar = out.Blob
	buf := bytes.Buffer{}
	err = oldProfile.MarshalCBOR(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal profile record: %w", err)
	}
	kv, err := atdata.UnmarshalCBOR(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to unmarhsal record CBOR: %w", err)
	}
	kv[constants.BlueskyProfileGoliveKey] = true
	kv["place.stream.stashedAvatar"] = oldAvatar
	inp := map[string]any{
		"collection": "app.bsky.actor.profile",
		"record":     kv,
		"rkey":       "self",
		"repo":       repoDID,
		"swapRecord": getRecordOut.Cid,
	}
	putOutput := comatproto.RepoPutRecord_Output{}
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &putOutput)
	if err != nil {
		return fmt.Errorf("failed to put profile record: %w", err)
	}
	log.Log(ctx, "uploaded redcircle", "ref", out.Blob.Ref.String(), "size", out.Blob.Size)
	return nil
}

func RestoreProfilePicture(ctx context.Context, repoDID string, client *oatproxy.XrpcClient, mod model.Model) error {
	oldProfile, err := mod.GetBskyProfile(ctx, repoDID, false)
	if err != nil {
		return fmt.Errorf("failed to get old profile: %w", err)
	}
	if oldProfile == nil {
		return fmt.Errorf("no old profile found for repoDID: %s", repoDID)
	}
	var getRecordOut comatproto.RepoGetRecord_Output
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       repoDID,
		"collection": "app.bsky.actor.profile",
		"rkey":       "self",
	}, nil, &getRecordOut)
	if err != nil {
		return fmt.Errorf("failed to get profile record: %w", err)
	}
	inp := comatproto.RepoPutRecord_Input{
		Collection: "app.bsky.actor.profile",
		Record:     &lexutil.LexiconTypeDecoder{Val: oldProfile},
		Rkey:       "self",
		Repo:       repoDID,
		SwapRecord: getRecordOut.Cid,
	}
	putOutput := comatproto.RepoPutRecord_Output{}
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &putOutput)
	if err != nil {
		return fmt.Errorf("failed to put profile record: %w", err)
	}
	log.Log(ctx, "restored profile picture", "cid", putOutput.Cid)
	return nil
}
