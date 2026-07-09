package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// SalvageSpools uploads live-rec spool data left behind by prior runs — node
// crashes, or streams that ended while their bucket was failing — so a
// recording survives even when the process didn't. It scans root
// (<data-dir>/live-rec-spool/<did>/<session>/) and uploads each leftover
// spool's segments as live-rec objects, splitting on livestream-URI changes
// and cutoverEvery of stream time just like the live path, with started-at
// taken from segment arrival times (spool file mtimes) — finalize orders
// objects by started-at, so salvaged objects sort into the right place no
// matter how late they upload. Drained spools are deleted.
//
// Run this once at startup, before stream sessions pile up. It's safe against
// concurrent new sessions because every session creates a fresh uniquely-named
// spool dir: anything present at startup is by construction orphaned. (A spool
// abandoned by a failing Close mid-run waits for the next restart; that needs
// a failure at the exact end of a stream, and the data just sits on disk in
// the meantime.)
//
// Failures leave the affected spool in place for the next attempt and move on
// to the next one; the returned error is only ever a scan-level failure.
func SalvageSpools(ctx context.Context, cfg Config, recorder Recorder, root string, keyPrefixFor func(did string) string, cutoverEvery time.Duration) error {
	return salvageSpools(ctx, NewClient(cfg), cfg.Bucket, recorder, root, keyPrefixFor, cutoverEvery)
}

func salvageSpools(ctx context.Context, client uploadAPI, bucket string, recorder Recorder, root string, keyPrefixFor func(did string) string, cutoverEvery time.Duration) error {
	ctx = log.WithLogValues(ctx, "func", "s3.SalvageSpools")
	didDirs, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil // no spool root: nothing was ever spooled
	}
	if err != nil {
		return fmt.Errorf("reading spool root %s: %w", root, err)
	}
	for _, didDir := range didDirs {
		if !didDir.IsDir() {
			continue
		}
		did := didDir.Name()
		sessions, err := os.ReadDir(filepath.Join(root, did))
		if err != nil {
			log.Error(ctx, "reading spool did dir", "did", did, "error", err)
			continue
		}
		for _, sess := range sessions {
			if !sess.IsDir() {
				continue
			}
			dir := filepath.Join(root, did, sess.Name())
			if err := salvageOne(ctx, client, bucket, recorder, dir, did, keyPrefixFor(did), cutoverEvery); err != nil {
				log.Error(ctx, "salvaging live-rec spool failed; leaving it for the next attempt", "dir", dir, "error", err)
			}
		}
		// Best-effort: remove the did dir if everything under it drained.
		_ = os.Remove(filepath.Join(root, did))
	}
	return nil
}

// salvageOne drains a single leftover spool into live-rec objects. Any error
// leaves the remaining segments on disk (already-completed objects stay
// completed — their segments were acked, so a re-run picks up exactly where
// this one failed).
func salvageOne(ctx context.Context, client uploadAPI, bucket string, recorder Recorder, dir, did, keyPrefix string, cutoverEvery time.Duration) error {
	spool, err := OpenSpool(dir, 0)
	if err != nil {
		return err
	}
	if spool.Len() == 0 {
		spool.Destroy()
		return nil
	}
	log.Log(ctx, "salvaging leftover live-rec spool", "dir", dir, "segments", spool.Len(), "bytes", spool.Bytes())

	// A bare uploader shell: objectWriter needs the client/recorder wiring but
	// no upload loop — salvage drives it synchronously.
	u := &S3Uploader{
		client:       client,
		bucket:       bucket,
		cutoverEvery: cutoverEvery,
		keyPrefix:    keyPrefix,
		userDID:      did,
		recorder:     recorder,
		spool:        spool,
	}
	w := &objectWriter{u: u}
	var nextSeq, lastConsumed int64 = 1, 0
	var salvaged int64
	for {
		seq, ok := spool.NextFrom(nextSeq)
		if !ok {
			break
		}
		data, arrived, uri, err := spool.Get(seq)
		if err != nil {
			log.Error(ctx, "skipping unreadable spool segment", "dir", dir, "seq", seq, "error", err)
			nextSeq = seq + 1
			continue
		}
		if w.current != nil && (arrived.Sub(w.current.started) >= cutoverEvery || uri != w.current.livestreamURI) {
			if err := w.complete(ctx); err != nil {
				w.abort(ctx, err)
				return err
			}
			spool.Ack(lastConsumed)
		}
		if w.current == nil {
			if err := w.start(ctx, uri, arrived); err != nil {
				return err
			}
		}
		if err := w.appendAndFlush(ctx, data); err != nil {
			w.abort(ctx, err)
			return err
		}
		salvaged += int64(len(data))
		lastConsumed = seq
		nextSeq = seq + 1
	}
	if err := w.complete(ctx); err != nil {
		w.abort(ctx, err)
		return err
	}
	spool.Ack(lastConsumed)
	spmetrics.S3LiveRecSpoolSalvagedBytesTotal.Add(float64(salvaged))
	log.Log(ctx, "salvaged live-rec spool", "dir", dir, "bytes", salvaged)
	spool.Destroy()
	return nil
}
