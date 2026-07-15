// Package upload implements resumable user uploads of arbitrary media
// content using the TUS protocol. Uploads are pre-created by the
// place.stream.media.createUpload XRPC (DPoP-authenticated) which returns a
// URL and a short-lived bearer token; clients then PATCH/HEAD chunks against
// the URL with only the bearer token, avoiding per-chunk DPoP.
//
// Two storage backends are supported. When S3 is configured, uploads are
// streamed directly into an S3 multipart upload, which lets any node in a
// station resume any upload. Otherwise uploads land on local disk under
// $DataDir/uploads — that mode requires a single-node deployment.
package upload

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	tusfilestore "github.com/tus/tusd/v2/pkg/filestore"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
	tusmemlocker "github.com/tus/tusd/v2/pkg/memorylocker"
	tuss3store "github.com/tus/tusd/v2/pkg/s3store"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// Backend identifies which TUS data store backs an upload.
type Backend string

const (
	BackendFile Backend = "file"
	BackendS3   Backend = "s3"
)

// TokenLifetime is how long an upload bearer token remains valid. Renewed
// uploads (long uploads, paused & resumed) will need a fresh token via
// createUpload — we deliberately keep this finite to limit token leak blast
// radius.
const TokenLifetime = 24 * time.Hour

// MaxUploadSize caps the upload size at the protocol layer. Set to zero to
// disable enforcement. Currently 50 GiB.
const MaxUploadSize int64 = 50 * 1024 * 1024 * 1024

// BasePath is the URL prefix the TUS handler is mounted at. Token claims
// embed the upload ID; URLs returned to clients are constructed from this
// prefix plus the assigned ID.
const BasePath = "/api/upload/"

const tokenAudience = "streamplace-upload"

// Manager owns the TUS store, the unrouted handler, and the bearer-token
// signing key. It's safe to use as a singleton across the lifetime of the
// server.
type Manager struct {
	cli      *config.CLI
	state    *statedb.StatefulDB
	composer *tushandler.StoreComposer
	handler  *tushandler.UnroutedHandler
	backend  Backend
	authKey  []byte
}

// New constructs an upload Manager, choosing the storage backend based on
// the CLI's S3 configuration. Returns an error when configured for a
// multi-node station without S3 — file-backed uploads cannot survive being
// served by a different node than the one that issued the upload URL.
func New(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB) (*Manager, error) {
	authKey, err := state.GetOrCreateUploadAuthKey()
	if err != nil {
		return nil, fmt.Errorf("upload: get auth key: %w", err)
	}

	multiNode := cli.ServerHost != "" && cli.BroadcasterHost != "" && cli.ServerHost != cli.BroadcasterHost
	if multiNode && !cli.S3Configured() {
		return nil, errors.New("upload: multi-node deployments require S3 storage; file-backed uploads are single-node only")
	}

	composer := tushandler.NewStoreComposer()
	tusmemlocker.New().UseIn(composer)

	var backend Backend
	if cli.S3Configured() {
		backend = BackendS3
		s3client := awss3.New(awss3.Options{
			Region: cli.S3Region,
			Credentials: credentials.NewStaticCredentialsProvider(
				cli.S3AccessKeyID,
				cli.S3SecretAccessKey,
				"",
			),
			BaseEndpoint: aws.String(cli.S3Endpoint),
			UsePathStyle: true,
		})
		store := tuss3store.New(cli.S3Bucket, s3client)
		store.ObjectPrefix = "uploads/"
		store.UseIn(composer)
		log.Log(ctx, "upload manager: S3 backend", "bucket", cli.S3Bucket)
	} else {
		backend = BackendFile
		dir := cli.DataFilePath([]string{"uploads"})
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("upload: create upload dir %q: %w", dir, err)
		}
		store := tusfilestore.New(dir)
		store.UseIn(composer)
		log.Log(ctx, "upload manager: file backend", "dir", dir)
	}

	cfg := tushandler.Config{
		StoreComposer:         composer,
		BasePath:              BasePath,
		MaxSize:               MaxUploadSize,
		NotifyCompleteUploads: true,
		// We pre-create uploads from the createUpload XRPC, so the protocol's
		// POST path is unused. Disabling download/termination keeps the
		// surface small; if we need cancellation we can flip DisableTermination.
		DisableDownload:    true,
		DisableTermination: false,
	}
	h, err := tushandler.NewUnroutedHandler(cfg)
	if err != nil {
		return nil, fmt.Errorf("upload: create tusd handler: %w", err)
	}

	m := &Manager{
		cli:      cli,
		state:    state,
		composer: composer,
		handler:  h,
		backend:  backend,
		authKey:  authKey,
	}

	go m.notifyLoop(ctx)
	return m, nil
}

// CreateResult is what the createUpload XRPC returns to the caller, modulo
// JSON field naming.
type CreateResult struct {
	UploadID    string
	UploadURL   string
	UploadToken string
	ExpiresAt   time.Time
}

// Create pre-creates a TUS upload for the given user and mints a bearer
// token bound to it. baseURL should be the absolute scheme+host the caller
// reached (e.g. "https://example.com") so the returned UploadURL is
// reachable for the client.
func (m *Manager) Create(ctx context.Context, did string, mimeType string, filename string, size int64, baseURL string) (*CreateResult, error) {
	if size <= 0 {
		return nil, errors.New("upload: size must be positive")
	}
	if MaxUploadSize > 0 && size > MaxUploadSize {
		return nil, fmt.Errorf("upload: size %d exceeds max %d", size, MaxUploadSize)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("upload: generate id: %w", err)
	}

	meta := tushandler.MetaData{
		"did":      did,
		"mimeType": mimeType,
	}
	if filename != "" {
		meta["filename"] = filename
	}
	info := tushandler.FileInfo{
		ID:       id.String(),
		Size:     size,
		MetaData: meta,
	}

	upload, err := m.composer.Core.NewUpload(ctx, info)
	if err != nil {
		return nil, fmt.Errorf("upload: create tusd upload: %w", err)
	}
	final, err := upload.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("upload: read back upload info: %w", err)
	}

	row := &statedb.Upload{
		ID:       final.ID,
		RepoDID:  did,
		MimeType: mimeType,
		Filename: filename,
		Size:     size,
		Backend:  string(m.backend),
	}
	if final.Storage != nil {
		// Filestore stores "Path"; s3store stores "Bucket"+"Key".
		if p, ok := final.Storage["Path"]; ok {
			row.Location = p
		} else if k, ok := final.Storage["Key"]; ok {
			row.Location = "s3://" + final.Storage["Bucket"] + "/" + k
		}
	}
	if err := m.state.CreateUpload(ctx, row); err != nil {
		return nil, fmt.Errorf("upload: persist upload row: %w", err)
	}

	expiresAt := time.Now().Add(TokenLifetime)
	tok, err := m.mintToken(did, final.ID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("upload: mint token: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + BasePath + final.ID
	return &CreateResult{
		UploadID:    final.ID,
		UploadURL:   url,
		UploadToken: tok,
		ExpiresAt:   expiresAt,
	}, nil
}

// ServeHTTP implements http.Handler for TUS requests. The caller is
// responsible for routing TUS-protocol methods (HEAD/PATCH/DELETE) to this
// handler at paths under BasePath. POST is rejected — uploads are
// pre-created exclusively via createUpload.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		http.Error(w, "POST not supported; call place.stream.media.createUpload to start an upload", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, BasePath)
	id = strings.Trim(id, "/")
	if id == "" {
		http.Error(w, "missing upload id in path", http.StatusBadRequest)
		return
	}

	if err := m.checkAuth(r, id); err != nil {
		log.Warn(r.Context(), "upload auth rejected", "error", err, "uploadId", id)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// tusd's handler reads the upload ID via strings.Trim(r.URL.Path, "/"),
	// so we need to strip BasePath before delegating. Cloning the request is
	// safer than mutating r.URL in-place.
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + id

	wrapped := m.handler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			m.handler.HeadFile(w, r)
		case http.MethodPatch:
			m.handler.PatchFile(w, r)
		case http.MethodDelete:
			m.handler.DelFile(w, r)
		case http.MethodOptions:
			// Middleware handles CORS; just terminate.
			w.WriteHeader(http.StatusOK)
		default:
			w.Header().Set("Allow", "HEAD, PATCH, DELETE, OPTIONS")
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	wrapped.ServeHTTP(w, r2)
}

// notifyLoop consumes upload completion events from tusd's hook channel and
// enqueues a VOD processing task for each finished upload.
func (m *Manager) notifyLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-m.handler.CompleteUploads:
			m.onComplete(ctx, ev)
		}
	}
}

func (m *Manager) onComplete(ctx context.Context, ev tushandler.HookEvent) {
	id := ev.Upload.ID
	ctx = log.WithLogValues(ctx, "uploadId", id)
	log.Log(ctx, "upload complete", "size", ev.Upload.Size)

	location := ""
	if ev.Upload.Storage != nil {
		if p, ok := ev.Upload.Storage["Path"]; ok {
			location = p
		} else if k, ok := ev.Upload.Storage["Key"]; ok {
			location = "s3://" + ev.Upload.Storage["Bucket"] + "/" + k
		}
	}
	if err := m.state.CompleteUpload(ctx, id, location); err != nil {
		log.Error(ctx, "failed to mark upload complete", "error", err)
	}

	row, err := m.state.GetUpload(ctx, id)
	if err != nil || row == nil {
		log.Error(ctx, "failed to look up completed upload", "error", err)
		return
	}
	payload := statedb.VODProcessTask{
		UploadID: row.ID,
		RepoDID:  row.RepoDID,
		MimeType: row.MimeType,
		Filename: row.Filename,
		Size:     row.Size,
		Backend:  row.Backend,
		Location: row.Location,
	}
	if _, err := m.state.EnqueueTask(ctx, statedb.TaskVODProcess, payload, statedb.WithTaskKey("vod-process:"+row.ID)); err != nil {
		log.Error(ctx, "failed to enqueue vod-process task", "error", err)
	}

	// Create a draft VOD in the 'processing' state, but only if the upload
	// isn't already associated with a pre-created draft. The modern client
	// flow creates the draft up front (place.stream.vod.createDraft) and passes
	// its URI to createUpload, which re-points the draft's origin_upload_id at
	// this upload — so a draft already exists for this row. Only fall back to
	// creating one here for legacy clients / external callers that didn't
	// supply a draftUri. A failed create is non-fatal: the upload still
	// processes.
	if existing, _ := m.state.GetDraftByUpload(ctx, row.ID); existing == nil {
		if _, err := m.state.CreateDraft(ctx, row.RepoDID, row.ID, &placestream.VodDraftVideo{
			LexiconTypeID: "place.stream.vod.draftVideo",
			Title:         filenameOrDefault(row.Filename),
			Status:        "processing",
			CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			log.Error(ctx, "failed to create draft for upload", "error", err)
		}
	}
}

// filenameOrDefault returns a human placeholder title for a freshly-uploaded
// draft: the original filename if present, else a generic "Uploaded video".
// The user edits this before publishing.
func filenameOrDefault(filename string) string {
	if filename != "" {
		return filename
	}
	return "Uploaded video"
}

func (m *Manager) mintToken(did, uploadID string, expiresAt time.Time) (string, error) {
	now := time.Now()
	tok, err := jwt.NewBuilder().
		Issuer("streamplace").
		Audience([]string{tokenAudience}).
		Subject(did).
		IssuedAt(now).
		Expiration(expiresAt).
		Claim("upload_id", uploadID).
		Build()
	if err != nil {
		return "", err
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.HS256, m.authKey))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

func (m *Manager) checkAuth(r *http.Request, uploadID string) error {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return errors.New("missing bearer token")
	}
	raw := strings.TrimPrefix(auth, "Bearer ")
	tok, err := jwt.Parse([]byte(raw),
		jwt.WithKey(jwa.HS256, m.authKey),
		jwt.WithValidate(true),
		jwt.WithAudience(tokenAudience),
	)
	if err != nil {
		return fmt.Errorf("parse token: %w", err)
	}
	v, ok := tok.Get("upload_id")
	if !ok {
		return errors.New("token missing upload_id claim")
	}
	claimID, ok := v.(string)
	if !ok || claimID != uploadID {
		return errors.New("token upload_id does not match request")
	}
	return nil
}
