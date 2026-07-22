package config

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/s3"
)

// spooledRecording is the S3-configured DebugRecordingFile: every write lands
// in the local spool file (the source of truth) and streams best-effort to S3.
// Close commits the upload and removes the spool on success; on any S3 failure
// the spool survives for SweepDebugRecordings to upload later. S3 trouble is
// therefore never a write/Close error — the recording is safe on disk.
type spooledRecording struct {
	ctx   context.Context
	local *os.File
	s3w   *s3.UploadWriter
	s3err error // first S3 failure; once set, S3 is out of the picture
}

func (w *spooledRecording) Name() string { return w.local.Name() }

func (w *spooledRecording) Write(p []byte) (int, error) {
	if w.s3err == nil {
		if _, err := w.s3w.Write(p); err != nil {
			w.s3err = err
			log.Error(w.ctx, "debug recording S3 upload failed mid-stream; keeping local spool", "path", w.local.Name(), "error", err)
			// The partial multipart upload is useless — the sweep re-uploads the
			// whole spool later. Abort is bounded by pkg/s3's per-op timeouts.
			if aerr := w.s3w.Abort(); aerr != nil {
				log.Error(w.ctx, "abort of failed debug recording upload", "error", aerr)
			}
		}
	}
	return w.local.Write(p)
}

func (w *spooledRecording) Close() error {
	lerr := w.local.Close()
	if w.s3err == nil {
		w.s3err = w.s3w.Close()
	}
	if w.s3err != nil {
		log.Error(w.ctx, "debug recording not committed to S3; keeping local spool for the salvage sweep", "path", w.local.Name(), "error", w.s3err)
		return lerr
	}
	// Committed to S3 — the spool has served its purpose. (Even if lerr != nil:
	// os.File writes are unbuffered syscalls, so a Close error doesn't mean the
	// S3 copy is short; keeping the spool would just make the sweep re-upload.)
	if err := os.Remove(w.local.Name()); err != nil {
		log.Warn(w.ctx, "could not remove committed debug recording spool", "path", w.local.Name(), "error", err)
	}
	log.Log(w.ctx, "debug recording committed to S3", "key", w.s3w.Name())
	return lerr
}

// debugRecordingSweepInterval is how often the sweeper looks for leftovers;
// debugRecordingSweepIdle is how long a file must sit unmodified before it's
// considered dead. An active recording's mtime advances with every write, and
// wedged workers are torn down by their watchdogs well inside the idle window,
// so anything idle this long is a leftover: a stalled/failed live upload, a
// crashed worker, or a recording from the era when S3 upload didn't work.
const (
	debugRecordingSweepInterval = 15 * time.Minute
	debugRecordingSweepIdle     = 15 * time.Minute
)

// DebugRecordingSweeper runs SweepDebugRecordings periodically (and once at
// startup) until ctx is done. Run it in main when S3 is configured — this is
// the salvage half of the local-spool durability story: whatever the live
// upload path couldn't commit, the sweep eventually does.
func (cli *CLI) DebugRecordingSweeper(ctx context.Context) error {
	ticker := time.NewTicker(debugRecordingSweepInterval)
	defer ticker.Stop()
	for {
		cli.SweepDebugRecordings(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// SweepDebugRecordings uploads leftover local debug recordings to S3 and
// removes them once committed. The on-disk layout under
// DataDir/debug-recordings mirrors the object-key layout exactly (both are
// ":"-sanitized), so a file's path relative to DataDir IS its key. Files
// modified within debugRecordingSweepIdle are skipped — they may still be
// written by an active worker.
func (cli *CLI) SweepDebugRecordings(ctx context.Context) {
	root := cli.DataFilePath([]string{"debug-recordings"})
	entries := []string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		if time.Since(info.ModTime()) < debugRecordingSweepIdle {
			return nil
		}
		entries = append(entries, path)
		return nil
	})
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error(ctx, "debug recording sweep: walk", "root", root, "error", err)
		}
		return
	}
	for _, path := range entries {
		if ctx.Err() != nil {
			return
		}
		if err := cli.sweepOneRecording(ctx, path); err != nil {
			log.Error(ctx, "debug recording sweep: upload failed; leaving spool for the next sweep", "path", path, "error", err)
		}
	}
}

func (cli *CLI) sweepOneRecording(ctx context.Context, path string) error {
	rel, err := filepath.Rel(cli.DataFilePath(nil), path)
	if err != nil {
		return err
	}
	key := filepath.ToSlash(rel)
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	w, err := s3.NewUploadWriter(ctx, s3.NewClient(cli.S3Config()), cli.S3Bucket, key, debugRecordingContentType(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, fd); err != nil {
		if aerr := w.Abort(); aerr != nil {
			log.Error(ctx, "debug recording sweep: abort", "key", key, "error", aerr)
		}
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	log.Log(ctx, "debug recording sweep: salvaged recording to S3", "key", key)
	return nil
}

func debugRecordingContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv":
		return "video/x-matroska"
	case ".cbor":
		return "application/cbor"
	}
	return "application/octet-stream"
}
