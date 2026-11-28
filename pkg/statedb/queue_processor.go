package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	glex "github.com/streamplace/glex/runtime"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/integrations/discord"
	"stream.place/streamplace/pkg/integrations/webhook"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/stationdb"

	"stream.place/streamplace/pkg/comatproto"
)

var TaskNotification = "notification"
var TaskChat = "chat"
var TaskStreamReceived = "stream_received"
var TaskFinalizeLivestream = "finalize_livestream"
var TaskFinalizeLivestreamVOD = "finalize_livestream_vod"
var TaskVODProcess = "vod_process"
var TaskViewCountAggregate = "view_count_aggregate"
var TaskCDNLogIngest = "cdn_log_ingest"
var TaskAutoPublishVOD = "auto_publish_vod"

// nonVODTaskTypes is every task type handled by the general queue worker.
// VOD processing runs on its own dedicated pool (see ProcessQueue) so a
// slow remux can't block these lighter tasks, so VOD is deliberately
// excluded here. Keep this list in sync when adding a new task type that
// is NOT VOD — anything missing from both this list and the VOD pool will
// never be dequeued.
var nonVODTaskTypes = []string{
	TaskNotification,
	TaskChat,
	TaskStreamReceived,
	TaskFinalizeLivestream,
	TaskViewCountAggregate,
	TaskCDNLogIngest,
	TaskAutoPublishVOD,
}

type NotificationTask struct {
	Livestream  placestream.Livestream_LivestreamView
	FeedPost    *appbsky.FeedDefs_PostView
	ChatProfile placestream.ChatProfile
	PDSURL      string
}

type ChatTask struct {
	MessageView placestream.ChatDefs_MessageView
}

type StreamReceivedTask struct {
	StreamerDID string `json:"streamerDID"`
}

type FinalizeLivestreamTask struct {
	LivestreamURI string `json:"livestreamURI"`
	// StaleSince is the record's lastSeenAt as it stood when this task was
	// rescheduled to give the heartbeat one more idle window. If it is still
	// that when the task runs again, the heartbeat is frozen and the record
	// is ended; without it the task rescheduled itself forever for a repo's
	// latest un-ended record.
	StaleSince string `json:"staleSince,omitempty"`
}

// heartbeatFrozen reports whether a rescheduled finalize found the record's
// heartbeat exactly where it left it a full idle window ago.
func heartbeatFrozen(task FinalizeLivestreamTask, lastSeenAt string) bool {
	return task.StaleSince != "" && task.StaleSince == lastSeenAt
}

// VODProcessTask is enqueued when a resumable upload completes.
type VODProcessTask struct {
	UploadID string `json:"uploadId"`
	RepoDID  string `json:"repoDID"`
	MimeType string `json:"mimeType"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size"`
	Backend  string `json:"backend"`
	Location string `json:"location"`
}

// FinalizeLivestreamVODTask is enqueued by the place.stream.media.finalizeLivestream
// procedure to turn a finished livestream's recorded MUXL objects into a VOD.
// UploadID is the synthetic Upload row the client polls (getUploadStatus) and
// then publishes (publishVideo), exactly as for a resumable upload.
type FinalizeLivestreamVODTask struct {
	UploadID      string `json:"uploadId"`
	RepoDID       string `json:"repoDID"`
	LivestreamURI string `json:"livestreamURI"`
	// LivestreamURIs, when set, are all the livestream records whose
	// recordings make up the VOD, in order (LivestreamURI is the first);
	// empty means just LivestreamURI.
	LivestreamURIs []string `json:"livestreamURIs,omitempty"`
	ObjectKeys     []string `json:"objectKeys,omitempty"`
	// Publish, when set, describes the place.stream.video record to
	// publish in the streamer's repo (with their stored session) as soon
	// as the VOD is finalized, instead of leaving a draft for them to
	// publish from the app (unless publishing fails). Set by the
	// operator's finalize route and by automatic VOD publishing
	// (AutoPublishVODTask).
	Publish *VideoDraft `json:"publish,omitempty"`
}

// VideoDraft is the part of a place.stream.video record known before the
// VOD exists: what the operator (or the livestream record) says about it.
// It is not a placestream.Video because that record's source union has no
// value until the finalizer has the MUXL CID, and an empty union will not
// marshal, so a Video cannot sit in a task payload; the publisher builds
// the record from this with Record() and fills in the source, duration and
// thumbnail from the finished upload.
type VideoDraft struct {
	Title       string                               `json:"title"`
	Description *string                              `json:"description,omitempty"`
	Tags        []string                             `json:"tags,omitempty"`
	Activity    *placestream.Video_Activity          `json:"activity,omitempty"`
	Connections []placestream.Video_Connections_Elem `json:"connections,omitempty"`
}

// Record is the place.stream.video record for the draft, minus the fields
// that come from the finalized upload.
func (d *VideoDraft) Record() *placestream.Video {
	return &placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         d.Title,
		Description:   d.Description,
		Tags:          d.Tags,
		Activity:      d.Activity,
		Connections:   d.Connections,
	}
}

// VideoPublisher journals each irreversible record write with RunTaskEffect.
// A successful return also requires any postpublication hosting intent to be durable.
type VideoPublisher func(ctx context.Context, t FinalizeLivestreamVODTask) (uri, cid string, err error)

func (state *StatefulDB) SetVideoPublisher(f VideoPublisher) {
	state.videoPublisher = f
}

// ViewCountAggregateTask is the payload for one aggregation window.
// Enqueued by every streamplace node at the configured interval; the
// unique task key (built from WindowStart/End) ensures only one node's
// enqueue + dequeue actually runs each window.
type ViewCountAggregateTask struct {
	WindowStart time.Time `json:"windowStart"`
	WindowEnd   time.Time `json:"windowEnd"`
}

// CDNLogIngestTask is the payload for one scheduled pass over the
// CDN's archived access logs. Tick is the UTC-aligned schedule slot;
// it's in the dedup key, not used by the ingester itself (which
// always looks for whatever is new).
type CDNLogIngestTask struct {
	Tick time.Time `json:"tick"`
}

func (state *StatefulDB) generalWorkerTaskTypes() []string {
	if state.CLI == nil || state.CLI.StationDID == "" ||
		(state.CLI.StationOriginPublisher && state.viewCountAggregator != nil) {
		return nonVODTaskTypes
	}
	return slices.DeleteFunc(slices.Clone(nonVODTaskTypes), func(kind string) bool {
		return kind == TaskViewCountAggregate
	})
}

// ProcessQueue runs the task queue until ctx is cancelled. VOD tasks are
// handled by a dedicated pool of vodConcurrency workers, so a slow remux
// can't block quick uploads behind it (or starve the lighter tasks);
// everything else runs on a single general worker. DequeueTask uses
// FOR UPDATE SKIP LOCKED on Postgres, so the workers never claim the same
// row. vodConcurrency is clamped to at least 1.
func (state *StatefulDB) ProcessQueue(ctx context.Context, vodConcurrency int) error {
	if vodConcurrency < 1 {
		vodConcurrency = 1
	}
	log.Log(ctx, "starting task queue", "vod_concurrency", vodConcurrency)

	group, ctx := errgroup.WithContext(ctx)

	// General worker: everything except VOD.
	group.Go(func() error {
		return state.runQueueWorker(ctx, "queue_processor", state.generalWorkerTaskTypes())
	})

	// Dedicated VOD pool. Live-to-VOD finalize also runs here: it's heavy
	// I/O (a full read of the recorded stream to hash + index it) that would
	// otherwise hog the single general worker and stall light tasks.
	for i := 0; i < vodConcurrency; i++ {
		workerID := fmt.Sprintf("vod_worker_%d", i)
		group.Go(func() error {
			return state.runQueueWorker(ctx, workerID, []string{TaskVODProcess, TaskFinalizeLivestreamVOD})
		})
	}

	return group.Wait()
}

// Empty dequeues and station commit contention back off on a timer or a queue poke.
func (state *StatefulDB) runQueueWorker(ctx context.Context, workerID string, taskTypes []string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		task, err := state.DequeueTask(ctx, workerID, taskTypes...)
		if err != nil {
			if !errors.Is(err, stationdb.ErrConflict) && !errors.Is(err, stationdb.ErrCommitUnknown) {
				return err
			}
			log.Debug(ctx, "queue dequeue did not confirm a claim", "err", err, "worker", workerID)
		} else if task != nil {
			if err := state.runClaimedTask(ctx, task, taskLeaseDuration/3); err != nil {
				log.Error(ctx, "queue task failed", "err", err, "worker", workerID, "taskId", task.ID)
			}
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		case <-state.pokeQueue:
		}
	}
}

const taskCloseTimeout = 5 * time.Second

func (state *StatefulDB) runClaimedTask(parent context.Context, task *AppTask, renewEvery time.Duration) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	ctx = context.WithValue(ctx, queueClaimKey{}, queueClaim{state, task.ID, task.ClaimToken})
	if err := CheckTaskClaim(ctx); err != nil {
		if parent.Err() != nil {
			closeCtx, stop := context.WithTimeout(context.WithoutCancel(parent), taskCloseTimeout)
			defer stop()
			return state.ReleaseTask(closeCtx, task.ID, task.ClaimToken)
		}
		return err
	}

	heartbeat, stopHeartbeat := context.WithCancel(ctx)
	joined := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(renewEvery)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeat.Done():
				joined <- nil
				return
			case <-ticker.C:
				if err := CheckTaskClaim(heartbeat); err != nil && heartbeat.Err() == nil {
					lost := fmt.Errorf("renew task claim: %w", err)
					cancel(lost)
					joined <- lost
					return
				}
			}
		}
	}()

	err := log.Recover(ctx, func() error { return state.processTask(ctx, task) })
	stopHeartbeat()
	if lost := <-joined; lost != nil {
		return errors.Join(lost, err)
	}
	closeCtx, stop := context.WithTimeout(context.WithoutCancel(parent), taskCloseTimeout)
	defer stop()
	switch {
	case err == nil:
		return state.CompleteTask(closeCtx, task.ID, task.ClaimToken)
	case errors.Is(err, ErrTaskClaimLost):
		return err
	case parent.Err() != nil:
		return errors.Join(err, state.ReleaseTask(closeCtx, task.ID, task.ClaimToken))
	case task.Type == TaskFinalizeLivestreamVOD && errors.Is(err, ErrTaskEffectUncertain):
		// A lost response may hide a committed receipt; block only for a durable reservation.
		return errors.Join(err, state.failTask(closeCtx, task.ID, task.ClaimToken, err.Error(), true))
	default:
		return errors.Join(err, state.FailTask(closeCtx, task.ID, task.ClaimToken, err.Error()))
	}
}

type queueClaimKey struct{}

type queueClaim struct {
	store interface {
		taskEffectStore
		RenewTask(context.Context, uint, string) error
	}
	id    uint
	token string
}

// CheckTaskClaim fences an idempotent queue write at its dispatch boundary.
func CheckTaskClaim(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if claim, ok := ctx.Value(queueClaimKey{}).(queueClaim); ok {
		renewCtx, cancel := context.WithTimeout(ctx, taskCloseTimeout)
		defer cancel()
		return claim.store.RenewTask(renewCtx, claim.id, claim.token)
	}
	return nil
}

// errTaskEffectNotDispatched allows retry only when delivery is known not to have occurred.
var errTaskEffectNotDispatched = errors.New("task effect not dispatched")

// ReadTaskEffect reads a confirmed receipt without reserving an effect.
// Outside the queue or for a missing effect it returns found=false.
// An unresolved reservation returns ErrTaskEffectUncertain.
func ReadTaskEffect[T any](ctx context.Context, key string) (receipt T, found bool, err error) {
	if err := ctx.Err(); err != nil {
		return receipt, false, err
	}
	claim, ok := ctx.Value(queueClaimKey{}).(queueClaim)
	if !ok {
		return receipt, false, nil
	}
	readCtx, stop := context.WithTimeout(ctx, taskCloseTimeout)
	raw, err := claim.store.readTaskEffect(readCtx, claim.id, claim.token, key)
	stop()
	if err != nil || raw == nil {
		return receipt, false, err
	}
	err = json.Unmarshal(raw, &receipt)
	return receipt, err == nil, err
}

// RunTaskEffect commits a reservation before dispatch and reuses a confirmed
// receipt on retries. A reservation without a receipt is never replayed.
// The callback must contain only the irreversible operation, not preparation.
func RunTaskEffect[T any](ctx context.Context, key string, send func(context.Context) (T, error)) (T, error) {
	return runTaskEffect(ctx, key, "", send)
}

// RunTaskEffectAtURI records the public record identity before writing to the PDS.
func RunTaskEffectAtURI[T any](ctx context.Context, key string, target syntax.ATURI, send func(context.Context) (T, error)) (T, error) {
	return runTaskEffect(ctx, key, target.String(), send)
}

func runTaskEffect[T any](ctx context.Context, key, target string, send func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	claim, ok := ctx.Value(queueClaimKey{}).(queueClaim)
	if !ok {
		return send(ctx) // synchronous non-queue caller
	}
	reserveCtx, stop := context.WithTimeout(ctx, taskCloseTimeout)
	prior, err := claim.store.beginTaskEffect(reserveCtx, claim.id, claim.token, key, target)
	stop()
	if err != nil {
		return zero, err
	}
	if prior != nil {
		err = json.Unmarshal(prior, &zero)
		return zero, err
	}
	// Use a bounded detached context for receipts on graceful cancellation.
	save := func(receipt []byte, retryAt time.Time) error {
		saveCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), taskCloseTimeout)
		defer stop()
		return claim.store.setTaskEffect(saveCtx, claim.id, claim.token, key, receipt, retryAt)
	}
	if err := ctx.Err(); err != nil {
		return zero, errors.Join(err, save(nil, time.Time{})) // no I/O was attempted
	}
	result, err := send(ctx)
	if errors.Is(err, errTaskEffectNotDispatched) {
		var retryAt time.Time
		var hint interface{ RetryAt() time.Time }
		if errors.As(err, &hint) {
			retryAt = hint.RetryAt()
		}
		return zero, errors.Join(err, save(nil, retryAt))
	}
	if err != nil {
		return zero, fmt.Errorf("%w (%s): %w", ErrTaskEffectUncertain, key, err)
	}
	receipt, err := json.Marshal(result)
	if err == nil {
		err = save(receipt, time.Time{})
	}
	if err != nil {
		return zero, fmt.Errorf("%w (%s): save receipt: %w", ErrTaskEffectUncertain, key, err)
	}
	return result, nil
}

// withTaskWrite serializes queue-derived SQL writes with takeover. The
// callback must contain only local SQL; never hold a claim lock across I/O.
func (state *StatefulDB) withTaskWrite(ctx context.Context, write func(*StatefulDB) error) error {
	claim, ok := ctx.Value(queueClaimKey{}).(queueClaim)
	if !ok {
		return write(state)
	}
	return state.withTaskClaimTx(ctx, claim.id, claim.token, func(tx *gorm.DB) error {
		if err := write(&StatefulDB{DB: tx, Type: state.Type, pokeQueue: state.pokeQueue}); err != nil {
			return err
		}
		return state.lockTaskClaim(tx, claim.id, claim.token)
	})
}

func (state *StatefulDB) processTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "taskType", task.Type, "taskId", fmt.Sprintf("%d", task.ID))
	switch task.Type {
	case TaskNotification:
		return state.processNotificationTask(ctx, task)
	case TaskChat:
		return state.processChatMessageTask(ctx, task)
	case TaskStreamReceived:
		return state.processStreamReceivedTask(ctx, task)
	case TaskFinalizeLivestream:
		return state.processFinalizeLivestreamTask(ctx, task)
	case TaskVODProcess:
		return state.processVODProcessTask(ctx, task)
	case TaskFinalizeLivestreamVOD:
		return state.processFinalizeLivestreamVODTask(ctx, task)
	case TaskViewCountAggregate:
		return state.processViewCountAggregateTask(ctx, task)
	case TaskCDNLogIngest:
		return state.processCDNLogIngestTask(ctx, task)
	case TaskAutoPublishVOD:
		return state.processAutoPublishVODTask(ctx, task)
	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}
}

// VODProcessor runs the gstreamer + muxl + S3 pipeline for one upload
// and returns the resulting BDASL CID. The function-pointer indirection
// keeps pkg/statedb from importing pkg/vod (which transitively pulls in
// gstreamer); the bootstrap (pkg/cmd) installs the concrete
// implementation at startup.
type VODProcessor func(ctx context.Context, t VODProcessTask) (cid string, err error)

func (state *StatefulDB) SetVODProcessor(f VODProcessor) { state.vodProcessor = f }

// SetNotifier installs the notifier after the DB is available for VAPID keys.
func (state *StatefulDB) SetNotifier(n notificationpkg.Notifier) { state.noter = n }

func (state *StatefulDB) processVODProcessTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processVODProcessTask")
	var t VODProcessTask
	if err := json.Unmarshal(task.Payload, &t); err != nil {
		return err
	}
	// Complete uploads from older clients have no explicit draft. Anchor one
	// before media processing so the terminal transition cannot pass it by.
	if err := state.withDraftTaskWrite(ctx, t.RepoDID, func(db *StatefulDB) error {
		_, err := state.CreateUploadDraftTx(ctx, db.DB, t.RepoDID, t.UploadID, nil)
		return err
	}); err != nil {
		return fmt.Errorf("prepare upload draft: %w", err)
	}
	return state.processQueueUpload(ctx, t.UploadID, true, func() (string, error) {
		if state.vodProcessor == nil {
			return "", errors.New("no VOD processor configured")
		}
		return state.vodProcessor(ctx, t)
	})
}

// LivestreamVODFinalizer concatenates a finished livestream's recorded MUXL
// objects into a content-addressed VOD blob and derives its sidecars without
// publishing it. Returns the resulting BDASL CID. Same
// function-pointer indirection as VODProcessor so pkg/statedb doesn't import
// the blob.Store/muxl-heavy pkg/vod.
type LivestreamVODFinalizer func(ctx context.Context, t FinalizeLivestreamVODTask) (cid string, err error)

func (state *StatefulDB) SetLivestreamVODFinalizer(f LivestreamVODFinalizer) {
	state.livestreamVODFinalizer = f
}

func (state *StatefulDB) processFinalizeLivestreamVODTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processFinalizeLivestreamVODTask")
	var t FinalizeLivestreamVODTask
	if err := json.Unmarshal(task.Payload, &t); err != nil {
		return err
	}
	if err := state.processQueueUpload(ctx, t.UploadID, t.Publish == nil, func() (string, error) {
		if state.livestreamVODFinalizer == nil {
			return "", errors.New("no livestream VOD finalizer configured")
		}
		return state.livestreamVODFinalizer(ctx, t)
	}); err != nil {
		return err
	}
	if t.Publish == nil {
		return nil
	}

	var err error
	if state.videoPublisher == nil {
		err = errors.New("no video publisher configured")
	} else {
		_, _, err = state.videoPublisher(ctx, t)
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrTaskClaimLost), errors.Is(err, ErrTaskEffectUncertain), ctx.Err() != nil, task.TryCount < task.MaxTries:
		return err
	}
	// Offer a manual draft only if no irreversible publication step ran.
	// A partial publish must not bypass the outbox via the manual path.
	return state.withDraftTaskWrite(ctx, t.RepoDID, func(db *StatefulDB) error {
		hasEffects, readErr := db.hasTaskEffects(ctx, task.ID, task.ClaimToken)
		if readErr != nil {
			return readErr
		}
		if hasEffects {
			return err
		}
		if _, createErr := state.CreateUploadDraftTx(ctx, db.DB, t.RepoDID, t.UploadID, LivestreamDraftRecord(t.Publish)); createErr != nil {
			return createErr
		}
		log.Warn(ctx, "automatic video publication failed before dispatch; leaving a draft", "uploadId", t.UploadID, "err", err)
		return nil
	})
}

func (state *StatefulDB) processQueueUpload(ctx context.Context, uploadID string, readyDraft bool, process func() (string, error)) error {
	upload, err := state.GetUpload(ctx, uploadID)
	if err != nil {
		return err
	}
	if upload == nil {
		return fmt.Errorf("queue upload %s not found", uploadID)
	}
	if upload.ProcessingStatus != "done" {
		if err := state.withTaskWrite(ctx, func(db *StatefulDB) error {
			return db.SetUploadProcessing(ctx, uploadID)
		}); err != nil {
			return err
		}
		if _, err := process(); err != nil {
			if ctx.Err() != nil || errors.Is(err, ErrTaskClaimLost) {
				return err
			}
			writeErr := state.withDraftTaskWrite(ctx, upload.RepoDID, func(db *StatefulDB) error {
				current, readErr := db.GetUpload(ctx, uploadID)
				if readErr != nil {
					return readErr
				}
				if current != nil && current.ProcessingStatus == "done" {
					return nil
				}
				if err := db.SetUploadFailed(ctx, uploadID, err.Error()); err != nil {
					return err
				}
				return db.syncDraftResult(ctx, uploadID)
			})
			return errors.Join(fmt.Errorf("process upload %s: %w", uploadID, err), writeErr)
		}
		upload, err = state.GetUpload(ctx, uploadID)
		if err != nil {
			return err
		}
		if upload == nil || upload.ProcessingStatus != "done" {
			return fmt.Errorf("queue upload %s did not persist its processing result", uploadID)
		}
	}
	if !readyDraft {
		return nil
	}
	return state.withDraftTaskWrite(ctx, upload.RepoDID, func(db *StatefulDB) error {
		return db.syncDraftResult(ctx, uploadID)
	})
}

// ViewCountAggregator runs the view-log → place.stream.media.viewCount
// aggregation for one window. Same function-pointer indirection trick
// as VODProcessor: pkg/statedb stays ignorant of pkg/viewlog (which
// pulls in blob storage + atproto publishing).
type ViewCountAggregator func(ctx context.Context, t ViewCountAggregateTask) error

func (state *StatefulDB) SetViewCountAggregator(f ViewCountAggregator) {
	state.viewCountAggregator = f
}

func (state *StatefulDB) processViewCountAggregateTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processViewCountAggregateTask")
	var t ViewCountAggregateTask
	if err := json.Unmarshal(task.Payload, &t); err != nil {
		return err
	}
	if state.viewCountAggregator == nil {
		return errors.New("no view-count aggregator configured")
	}
	if err := state.viewCountAggregator(ctx, t); err != nil {
		return fmt.Errorf("view-count aggregation: %w", err)
	}
	return nil
}

// CDNLogIngester pulls newly archived CDN access logs into the view-log
// store. Same indirection as ViewCountAggregator.
type CDNLogIngester func(ctx context.Context) error

func (state *StatefulDB) SetCDNLogIngester(f CDNLogIngester) {
	state.cdnLogIngester = f
}

func (state *StatefulDB) processCDNLogIngestTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processCDNLogIngestTask")
	if state.cdnLogIngester == nil {
		return errors.New("no CDN log ingester configured")
	}
	if err := state.cdnLogIngester(ctx); err != nil {
		return fmt.Errorf("cdn log ingest: %w", err)
	}
	return nil
}

func (state *StatefulDB) processFinalizeLivestreamTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processFinalizeLivestreamTask")
	log.Debug(ctx, "processing finalize livestream task")
	var finalizeLivestreamTask FinalizeLivestreamTask
	if err := json.Unmarshal(task.Payload, &finalizeLivestreamTask); err != nil {
		return err
	}
	livestream, err := state.model.GetLivestream(finalizeLivestreamTask.LivestreamURI)
	if err != nil {
		return fmt.Errorf("failed to get latest livestream for userDID: %w", err)
	}
	if livestream == nil {
		return fmt.Errorf("no livestream found for URI: %s", finalizeLivestreamTask.LivestreamURI)
	}
	lastLivestreamView, err := livestream.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("failed to convert livestream to streamplace livestream: %w", err)
	}
	rec, ok := lastLivestreamView.Record.Val.(*placestream.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}
	if rec.LastSeenAt == nil {
		return fmt.Errorf("livestream has no last seen at")
	}
	lastSeenTime, err := time.Parse(time.RFC3339, *rec.LastSeenAt)
	if err != nil {
		return fmt.Errorf("could not parse last seen at: %w", err)
	}
	if rec.IdleTimeoutSeconds == nil || *rec.IdleTimeoutSeconds == 0 {
		log.Debug(ctx, "livestream has no idle timeout, skipping finalization", "uri", livestream.URI)
		return nil
	}
	if time.Since(lastSeenTime) < (time.Duration(*rec.IdleTimeoutSeconds) * time.Second) {
		log.Debug(ctx, "livestream is active, skipping finalization", "lastSeenAt", lastSeenTime)
		return nil
	}
	// If this record is still the streamer's latest livestream, do NOT end it
	// on a stale lastSeenAt alone. lastSeenAt only advances via the per-segment
	// heartbeat (StreamSession.doUpdateLivestream), which is coupled to segment
	// arrival and can lag behind actual ingestion — e.g. after an ingest gap
	// long enough to tear down and restart the StreamSession, the new session's
	// heartbeat may not land on this record before the idle timer fires. Ending
	// here would set endedAt on the record the active stream is publishing
	// under, taking the stream pre-live underneath a still-flowing ingest.
	//
	// Instead, reschedule the check for one more idle window: if the stream is
	// truly abandoned the heartbeat stays frozen and we end it on the next
	// pass; if it's a heartbeat-lag artifact, the heartbeat catches up and the
	// rescheduled task hits the "active" early-return above.
	//
	// BUT the heartbeat-lag guard only applies to repos this node is actively
	// ingesting — the heartbeat runs inside StreamSession.doUpdateLivestream,
	// which requires the streamer to have a local OAuth session on this node.
	// If no session exists, there is no heartbeat to wait for and no write
	// access to end the record anyway: the record arrived via firehose sync
	// from an account that never connected here. Rescheduling in that case just
	// respawns the task every idle window forever (the rescheduled key embeds a
	// fresh timestamp, so dedup never fires), flooding the logs and growing the
	// task table without bound. So drop the task instead.
	latest, err := state.model.GetLatestLivestreamForRepo(livestream.RepoDID)
	if err != nil {
		return fmt.Errorf("failed to get latest livestream for repo: %w", err)
	}
	if latest != nil && latest.URI == livestream.URI {
		// Check for a local session before rescheduling. GetSessionByDID
		// returns gorm.ErrRecordNotFound (or nil session via callers that
		// swallow it) when the repo has never logged in here.
		session, err := state.GetSessionByDID(livestream.RepoDID)
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && session == nil) {
			log.Debug(ctx, "stale latest livestream has no local session; dropping finalize task (firehose-observed, no heartbeat to wait for)", "uri", livestream.URI, "lastSeenAt", lastSeenTime)
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get session for finalize-livestream guard: %w", err)
		}
		if heartbeatFrozen(finalizeLivestreamTask, *rec.LastSeenAt) {
			log.Log(ctx, "livestream is latest for repo and its heartbeat has not moved in a full idle window; ending it", "uri", livestream.URI, "lastSeenAt", lastSeenTime)
		} else {
			rescheduledAt := time.Now().Add(time.Duration(*rec.IdleTimeoutSeconds) * time.Second).UTC()
			rescheduledKey := fmt.Sprintf("finalize-livestream::%s::retry-%d", livestream.URI, task.ID)
			next := finalizeLivestreamTask
			next.StaleSince = *rec.LastSeenAt
			if err := state.withTaskWrite(ctx, func(db *StatefulDB) error {
				_, err := db.EnqueueTask(ctx, TaskFinalizeLivestream, next, WithTaskKey(rescheduledKey), WithScheduledAt(rescheduledAt))
				return err
			}); err != nil {
				return fmt.Errorf("failed to reschedule finalize livestream task: %w", err)
			}
			log.Log(ctx, "livestream is latest for repo but lastSeenAt is stale; rescheduling finalize once to let heartbeat catch up", "uri", livestream.URI, "lastSeenAt", lastSeenTime, "rescheduledAt", rescheduledAt)
			return nil
		}
	}
	return state.EndLivestreamRecord(ctx, livestream, rec)
}

// EndLivestreamRecord sets endedAt (= lastSeenAt) on a livestream record in
// the streamer's repo, with the streamer's stored OAuth session. Used by the
// idle finalize task and by the operator's finalize route for a record the
// streamer never stopped.
func (state *StatefulDB) EndLivestreamRecord(ctx context.Context, livestream *model.Livestream, rec *placestream.Livestream) error {
	session, err := state.GetSessionByDID(livestream.RepoDID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	session, err = state.OATProxy.RefreshIfNeeded(session)
	if err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}
	client, err := state.OATProxy.GetXrpcClient(session)
	if err != nil {
		return fmt.Errorf("failed to get xrpc client: %w", err)
	}
	if rec.EndedAt != nil {
		log.Debug(ctx, "livestream has already ended, skipping", "uri", livestream.URI, "endedAt", *rec.EndedAt)
		return nil
	}

	uri, err := syntax.ParseATURI(livestream.URI)
	if err != nil {
		return fmt.Errorf("failed to parse ATURI: %w", err)
	}

	rec.EndedAt = rec.LastSeenAt

	inp := comatproto.RepoPutRecord_Input{
		Collection: "place.stream.livestream",
		Record:     &glex.LexiconTypeDecoder{Val: rec},
		Rkey:       uri.RecordKey().String(),
		Repo:       livestream.RepoDID,
		SwapRecord: &livestream.CID,
	}
	out := comatproto.RepoPutRecord_Output{}

	if err := CheckTaskClaim(ctx); err != nil {
		return err
	}
	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out)
	if err != nil {
		return fmt.Errorf("failed to update livestream record: %w", err)
	}

	log.Log(ctx, "livestream finalized", "uri", livestream.URI, "endedAt", *rec.EndedAt)

	return nil
}

func (state *StatefulDB) processNotificationTask(ctx context.Context, task *AppTask) error {
	var notificationTask NotificationTask
	if err := json.Unmarshal(task.Payload, &notificationTask); err != nil {
		return err
	}
	lsv := notificationTask.Livestream
	rec, err := glex.RecordAs[placestream.Livestream](lsv.Record.Val)
	if err != nil {
		return fmt.Errorf("invalid livestream record: %w", err)
	}
	userDID := lsv.Author.Did

	pushErr := state.sendTaskPushes(ctx, userDID, lsv.Author.Handle, rec.Title)
	if errors.Is(pushErr, ErrTaskClaimLost) || ctx.Err() != nil {
		return errors.Join(pushErr, ctx.Err())
	}
	hookErr := state.sendTaskWebhooks(ctx, userDID, "livestream", func(ctx context.Context, w *placestream.ServerDefs_Webhook) error {
		return webhook.SendLivestreamWebhook(ctx, w, notificationTask.PDSURL, &lsv, notificationTask.FeedPost, &notificationTask.ChatProfile)
	})
	return errors.Join(pushErr, hookErr)
}

func (state *StatefulDB) sendTaskPushes(ctx context.Context, userDID, handle, title string) error {
	followers, err := state.model.GetUserFollowers(ctx, userDID)
	if err != nil {
		return err
	}
	dids := make([]string, 0, len(followers))
	for _, follower := range followers {
		dids = append(dids, follower.UserDID)
	}
	notifications, err := state.GetManyNotifications(dids)
	if err != nil || len(notifications) == 0 {
		return err
	}
	if state.noter == nil {
		return errors.New("no push notifier configured")
	}
	user := model.HandleOrDID(handle, userDID)
	name := user
	if model.IsUsableHandle(handle) {
		name = "@" + name
	}
	blast := &notificationpkg.NotificationBlast{
		Title: fmt.Sprintf("🔴 %s is LIVE!", name),
		Body:  title,
		Data:  map[string]string{"path": "/" + user},
	}
	group, pushCtx := errgroup.WithContext(ctx)
	group.SetLimit(taskPushConcurrency)
	var mu sync.Mutex
	var errs []error
	for _, n := range notifications {
		if pushCtx.Err() != nil {
			break
		}
		group.Go(func() error {
			if err := pushCtx.Err(); err != nil {
				return err
			}
			err := log.Recover(pushCtx, func() error { return state.sendTaskPush(pushCtx, n, blast) })
			if errors.Is(err, ErrTaskClaimLost) {
				return err
			}
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
			return nil
		})
	}
	err = group.Wait()
	return errors.Join(err, ctx.Err(), errors.Join(errs...))
}

const taskPushConcurrency = 8

func (state *StatefulDB) sendTaskPush(ctx context.Context, n Notification, blast *notificationpkg.NotificationBlast) error {
	if router, ok := state.noter.(*notificationpkg.MultiNotifier); ok && !router.Supports(n.Type) {
		return fmt.Errorf("no push transport configured for %s", n.Type)
	}
	target := [1]notificationpkg.NotificationTarget{{Token: n.Token, Type: n.Type}}
	expired, err := RunTaskEffect(ctx, taskEffectKey("push", n.Token), func(ctx context.Context) (bool, error) {
		err := state.noter.Blast(ctx, target[:], blast)
		if len(notificationpkg.ExpiredTokens(err)) > 0 && !errors.Is(err, notificationpkg.ErrPushNotDispatched) {
			return true, nil // a definite expired-subscription response
		}
		if errors.Is(err, notificationpkg.ErrPushNotDispatched) {
			return false, fmt.Errorf("%w: %w", errTaskEffectNotDispatched, err)
		}
		return false, err
	})
	if errors.Is(err, ErrTaskEffectUncertain) && !errors.Is(err, ErrTaskClaimLost) {
		log.Error(ctx, "push delivery unconfirmed", "err", err)
		return nil // The unresolved reservation keeps the task from completing.
	}
	if err == nil && expired {
		return state.withTaskWrite(ctx, func(db *StatefulDB) error { return db.DeleteNotification(n.Token) })
	}
	return err
}

func (state *StatefulDB) processStreamReceivedTask(ctx context.Context, task *AppTask) error {
	var streamReceivedTask StreamReceivedTask
	if err := json.Unmarshal(task.Payload, &streamReceivedTask); err != nil {
		return err
	}
	return state.sendTaskWebhooks(ctx, streamReceivedTask.StreamerDID, "stream.received", func(ctx context.Context, w *placestream.ServerDefs_Webhook) error {
		return webhook.SendStreamReceivedWebhook(ctx, w, streamReceivedTask.StreamerDID)
	})
}

func (state *StatefulDB) processChatMessageTask(ctx context.Context, task *AppTask) error {
	var chatTask ChatTask
	if err := json.Unmarshal(task.Payload, &chatTask); err != nil {
		return err
	}
	scm := chatTask.MessageView
	rec, err := glex.RecordAs[placestream.ChatMessage](scm.Record.Val)
	if err != nil {
		return fmt.Errorf("invalid chat message record: %w", err)
	}

	return state.sendTaskWebhooks(ctx, rec.Streamer, "chat", func(ctx context.Context, w *placestream.ServerDefs_Webhook) error {
		return webhook.SendChatWebhook(ctx, w, scm.Author.Did, &scm)
	})
}

func (state *StatefulDB) sendTaskWebhooks(ctx context.Context, did, event string, send func(context.Context, *placestream.ServerDefs_Webhook) error) error {
	hooks, err := state.GetActiveWebhooksForUser(did, event)
	if err != nil {
		return fmt.Errorf("list %s webhooks: %w", event, err)
	}
	var errs []error
	for _, w := range hooks {
		if err := ctx.Err(); err != nil {
			return err
		}
		target, err := w.ToLexicon()
		if err != nil {
			errs = append(errs, fmt.Errorf("decode webhook %s: %w", w.ID, err))
			continue
		}
		attempted := false
		dispatch := discord.WithWebhookDispatch(ctx, func(ctx context.Context, post func(context.Context) error) error {
			_, err := RunTaskEffect(ctx, taskEffectKey("webhook", w.ID), func(ctx context.Context) (bool, error) {
				attempted = true
				err := post(ctx)
				if errors.Is(err, discord.ErrWebhookRateLimited) {
					err = fmt.Errorf("%w: %w", errTaskEffectNotDispatched, err)
				}
				return true, err
			})
			return err
		})
		err = send(dispatch, &target)
		if errors.Is(err, ErrTaskClaimLost) || ctx.Err() != nil {
			return errors.Join(err, ctx.Err())
		}
		if attempted {
			if statErr := state.withTaskWrite(ctx, func(db *StatefulDB) error {
				if err != nil {
					return db.IncrementWebhookError(w.ID)
				}
				return db.ResetWebhookError(w.ID)
			}); statErr != nil {
				log.Error(ctx, "update webhook error count", "webhook_id", w.ID, "err", statErr)
			}
		}
		if errors.Is(err, ErrTaskEffectUncertain) {
			log.Error(ctx, "webhook delivery unconfirmed", "webhook_id", w.ID, "err", err)
		} else if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
