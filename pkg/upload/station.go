package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
	tusmemlocker "github.com/tus/tusd/v2/pkg/memorylocker"
	"gorm.io/gorm"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/stationdb"
)

// NewStation uses the supplied station's shared, durable SQL database and
// private S3 storage. It does not change how New handles standalone uploads.
func NewStation(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB) (*Manager, error) {
	if !cli.S3Configured() {
		return nil, errors.New("upload: station uploads require shared S3 storage")
	}
	prefix, _, err := cli.StationStorage()
	if err != nil {
		return nil, fmt.Errorf("upload: resolve station storage: %w", err)
	}
	cfg := cli.S3Config()
	objects := stationS3Store{blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket)}
	return newStationManager(ctx, state, objects, BackendS3, prefix)
}

type stationS3Store struct{ *blob.S3Store }

func (s stationS3Store) NewWriter(ctx context.Context, key, contentType string) (blob.Writer, error) {
	// Unique candidate keys are invisible until Complete; no staging copy is needed.
	return s3.NewMultipartWriter(ctx, s.Client(), s.Bucket(), key, contentType)
}

func newStationManager(ctx context.Context, state *statedb.StatefulDB, objects blob.Store, backend Backend, prefix string) (*Manager, error) {
	err := state.MigrateStationUploads(ctx)
	if errors.Is(err, stationdb.ErrConflict) {
		err = state.MigrateStationUploads(ctx) // Idempotent DDL; the first attempt rolled back.
	}
	if err != nil {
		return nil, fmt.Errorf("upload: migrate station uploads: %w", err)
	}
	authKey, err := state.GetOrCreateUploadAuthKey()
	if err != nil {
		return nil, fmt.Errorf("upload: get auth key: %w", err)
	}
	store := &stationStore{ctx: ctx, state: state, objects: objects, backend: backend, objectPrefix: prefix + "/uploads/"}
	composer := tushandler.NewStoreComposer()
	composer.UseCore(store)
	composer.UseTerminater(store)
	// Local lock only; no lock is held in shared state during any transfer.
	tusmemlocker.New().UseIn(composer)
	h, err := tushandler.NewUnroutedHandler(tushandler.Config{
		StoreComposer:   composer,
		BasePath:        BasePath,
		MaxSize:         MaxUploadSize,
		DisableDownload: true,
	})
	if err != nil {
		return nil, fmt.Errorf("upload: create station tusd handler: %w", err)
	}
	return &Manager{state: state, composer: composer, handler: h, backend: backend, authKey: authKey, station: store}, nil
}

const stationTransferTimeout = 2 * time.Hour

// Offline GC: stop all upload writers for the namespace, including paused
// handlers on other nodes, and let outstanding SQL and S3 operations settle.
// Sweep only stations/<namespace>/uploads/. Keep keys in uploads.location and
// parts joined to a live cursor for an upload with no completed_at; prune
// retired part rows after sweeping. Never expire completed objects by age.
// Use S3 AbortIncompleteMultipartUpload for abandoned MPUs under that prefix.
// Station uploads do not use bucket-global .staging/. A namespace sweep must
// leave it alone; sweep it only with all blob writers in the bucket quiesced.
type stationStore struct {
	ctx          context.Context
	state        *statedb.StatefulDB
	objects      blob.Store
	backend      Backend
	objectPrefix string
}

type stationUpload struct {
	store *stationStore
	row   *statedb.StationUpload
}

func (s *stationStore) NewUpload(ctx context.Context, info tushandler.FileInfo) (tushandler.Upload, error) {
	row := &statedb.StationUpload{Upload: statedb.Upload{
		ID:       info.ID,
		RepoDID:  info.MetaData["did"],
		MimeType: info.MetaData["mimeType"],
		Filename: info.MetaData["filename"],
		Size:     info.Size,
		Backend:  string(s.backend),
	}}
	if err := s.state.CreateStationUpload(ctx, &row.Upload); err != nil {
		return nil, err
	}
	return &stationUpload{store: s, row: row}, nil
}

func (s *stationStore) GetUpload(ctx context.Context, id string) (tushandler.Upload, error) {
	row, err := s.state.GetStationUpload(ctx, id)
	if err != nil {
		return nil, stationTUSError(ctx, err)
	}
	return &stationUpload{store: s, row: row}, nil
}

func (s *stationStore) AsTerminatableUpload(u tushandler.Upload) tushandler.TerminatableUpload {
	return u.(*stationUpload)
}

func (u *stationUpload) GetInfo(context.Context) (tushandler.FileInfo, error) {
	meta := tushandler.MetaData{"did": u.row.RepoDID, "mimeType": u.row.MimeType}
	if u.row.Filename != "" {
		meta["filename"] = u.row.Filename
	}
	return tushandler.FileInfo{ID: u.row.ID, Size: u.row.Size, Offset: u.row.Offset, MetaData: meta}, nil
}

func (u *stationUpload) GetReader(ctx context.Context) (io.ReadCloser, error) {
	if u.row.Offset != u.row.Size {
		return nil, tushandler.ErrUploadNotFinished
	}
	key, ok := u.store.objects.ParseLocation(u.row.Location)
	if !ok {
		return nil, errors.New("upload: station object outside configured store")
	}
	r, err := u.store.objects.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{io.NewSectionReader(r, 0, r.Size()), r}, nil
}

func (u *stationUpload) Terminate(ctx context.Context) error {
	return stationTUSError(ctx, u.store.state.TerminateStationUpload(ctx, u.row.ID))
}

func (u *stationUpload) FinishUpload(context.Context) error {
	// Returning the final byte count from WriteChunk already committed the
	// object, offset and outbox. tusd must not initiate a second publish here.
	if u.row.Offset != u.row.Size {
		return tushandler.ErrUploadNotFinished
	}
	return nil
}

type stationPatchKey struct{}

// tusd's reader converts body failures to EOF so stores can keep partial
// writes. Observe the raw body so station PATCHes publish only clean reads.
type stationPatch struct {
	io.ReadCloser
	ctx    context.Context
	res    *http.ResponseController
	length int64
	read   int64
	err    error
}

func observeStationPatch(w http.ResponseWriter, r *http.Request) *http.Request {
	body := &stationPatch{ReadCloser: r.Body, ctx: r.Context(), res: http.NewResponseController(w), length: r.ContentLength}
	r.Body = body
	return r.WithContext(context.WithValue(r.Context(), stationPatchKey{}, body))
}

func (p *stationPatch) Read(dst []byte) (int, error) {
	n, err := p.ReadCloser.Read(dst)
	p.read += int64(n)
	p.err = err
	return n, err
}

func (u *stationUpload) WriteChunk(tusCtx context.Context, offset int64, src io.Reader) (int64, error) {
	if offset != u.row.Offset {
		return 0, tushandler.ErrMismatchOffset
	}
	patch, ok := tusCtx.Value(stationPatchKey{}).(*stationPatch)
	if !ok {
		return 0, errors.New("upload: missing station PATCH body")
	}
	if patch.length == 0 {
		return 0, nil
	}
	if patch.length > u.row.Size-offset {
		return 0, tushandler.ErrSizeExceeded
	}
	ctx, cancel := context.WithTimeout(patch.ctx, stationTransferTimeout)
	defer cancel()
	stop := context.AfterFunc(u.store.ctx, cancel)
	defer stop()
	if err := u.store.ctx.Err(); err != nil {
		return 0, tushandler.ErrServerShutdown
	}

	part, err := u.stageChunk(ctx, src, patch)
	if err != nil || part.Size == 0 {
		return 0, stationTUSError(ctx, err)
	}
	end := offset + part.Size
	if end < u.row.Size {
		err = u.store.state.AppendStationUploadPart(ctx, part)
	} else {
		key := part.ObjectKey
		if offset > 0 {
			key, err = u.assemble(ctx, part)
			if err != nil {
				return 0, stationTUSError(ctx, err)
			}
		}
		location := u.store.objects.URL(key)
		err = u.store.state.CompleteStationUpload(ctx, u.row, location)
		if err == nil {
			u.row.Location = location
		}
	}
	if err != nil {
		// A failed reply is not proof that SQL rolled back. Never delete an
		// object here. HEAD will see the old offset or the durable new one.
		return 0, stationTUSError(ctx, err)
	}
	u.row.Offset = end
	return part.Size, nil
}

func (u *stationUpload) newObjectKey() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return u.store.objectPrefix + u.row.ID + "/" + id.String(), nil
}

func (u *stationUpload) stageChunk(ctx context.Context, src io.Reader, patch *stationPatch) (statedb.StationUploadPart, error) {
	var part statedb.StationUploadPart
	key, err := u.newObjectKey()
	if err != nil {
		return part, err
	}
	w, err := u.store.objects.NewWriter(ctx, key, "application/octet-stream")
	if err != nil {
		return part, err
	}
	defer w.Close()
	n, err := io.Copy(w, src)
	if err != nil {
		return part, err
	}
	if patch.read != n || n > u.row.Size-u.row.Offset {
		return part, tushandler.ErrSizeExceeded // MaxBytesReader consumed an overflow byte.
	}
	if patch.err != nil && patch.err != io.EOF && patch.err != http.ErrBodyReadAfterClose {
		return part, nil // tusd already captured the transport error for the response.
	}
	if patch.err != io.EOF || (patch.length >= 0 && patch.length != n) {
		return part, tushandler.ErrUnexpectedEOF
	}
	if n == 0 {
		return part, nil
	}
	// tusd's deadline is for the HTTP body; a large S3 join can take longer.
	deadline, _ := ctx.Deadline()
	_ = patch.res.SetWriteDeadline(deadline)
	if err := w.Complete(); err != nil {
		return part, err
	}
	return statedb.StationUploadPart{UploadID: u.row.ID, Offset: u.row.Offset, Size: n, ObjectKey: key}, nil
}

func (u *stationUpload) assemble(ctx context.Context, tail statedb.StationUploadPart) (string, error) {
	key, err := u.newObjectKey()
	if err != nil {
		return "", err
	}
	w, err := u.store.objects.NewWriter(ctx, key, u.row.MimeType)
	if err != nil {
		return "", err
	}
	defer w.Close()
	buf := make([]byte, 128*1024)
	for offset := int64(0); offset < tail.Offset; {
		parts, err := u.store.state.ListStationUploadParts(ctx, u.row.ID, offset, tail.Offset)
		if err != nil {
			return "", err
		}
		if len(parts) == 0 {
			return "", fmt.Errorf("upload: missing station chunk at offset %d", offset)
		}
		for _, part := range parts {
			if part.Offset != offset || part.Size <= 0 || part.Size > tail.Offset-offset {
				return "", fmt.Errorf("upload: invalid station chunk at offset %d", offset)
			}
			if err := u.copyPart(ctx, w, part, buf); err != nil {
				return "", err
			}
			offset += part.Size
		}
	}
	if err := u.copyPart(ctx, w, tail, buf); err != nil {
		return "", err
	}
	return key, w.Complete()
}

func (u *stationUpload) copyPart(ctx context.Context, w io.Writer, part statedb.StationUploadPart, buf []byte) error {
	r, err := u.store.objects.Open(ctx, part.ObjectKey)
	if err != nil {
		return err
	}
	defer r.Close()
	if r.Size() != part.Size {
		return fmt.Errorf("upload: wrong blob size for station chunk at offset %d", part.Offset)
	}
	n, err := io.CopyBuffer(w, io.NewSectionReader(r, 0, part.Size), buf)
	if err != nil {
		return err
	}
	if n != part.Size {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func stationTUSError(ctx context.Context, err error) error {
	var tusErr tushandler.Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tushandler.ErrNotFound
	case errors.Is(err, statedb.ErrStationUploadOffset):
		return tushandler.ErrMismatchOffset
	case errors.Is(err, stationdb.ErrConflict), errors.Is(err, stationdb.ErrCommitUnknown):
		return tushandler.NewError("ERR_STATION_UPLOAD_RETRY", "station upload write did not return a confirmed result; check HEAD before resubmitting", http.StatusServiceUnavailable)
	case errors.As(err, &tusErr):
		return tusErr
	default:
		log.Error(ctx, "station upload failed", "error", err)
		return tushandler.NewError("ERR_STATION_UPLOAD", "station upload failed; check the upload offset before retrying", http.StatusInternalServerError)
	}
}
