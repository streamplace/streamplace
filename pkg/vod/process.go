// Package vod implements the post-upload processing pipeline for a VOD
// upload: read the user's raw file from wherever the upload manager
// stored it, run it through gstreamer (parsebin -> mp4mux -> muxl
// concatenator), and write the resulting fMP4 to S3 under a key derived
// from the BLAKE3-based BDASL CID of the final bytes.
//
// The full pipeline is streaming end-to-end — bytes flow from the source
// (file or ranged S3 GETs), through gstreamer's appsrc -> parsebin ->
// fdkaacenc/h264parse -> mp4mux -> appsink, into the muxl wasm
// concatenator, and out to an S3 multipart upload. The bdasl hasher
// runs as a tee on the way out; the final CID is known only when the
// last byte is written.
package vod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	s3pkg "stream.place/streamplace/pkg/s3"
)

// Input is the per-upload state needed to drive ProcessVOD. It's a
// flattened copy of statedb.VODProcessTask so this package doesn't
// import statedb (which would invite a dependency cycle with the queue
// processor).
type Input struct {
	UploadID string
	RepoDID  string
	MimeType string
	Filename string
	Size     int64
	// Backend is the storage tier the user upload lives on: "file" or "s3".
	Backend string
	// Location is the path (file backend) or s3:// URL (s3 backend) of
	// the user upload.
	Location string
}

// Backend constants mirror pkg/upload.BackendFile / BackendS3 without
// importing the upload package (which would pull statedb in transitively).
const (
	BackendFile = "file"
	BackendS3   = "s3"
)

// StagingPrefix is where in-progress VOD outputs land in S3 before being
// renamed to their content-addressed key. We park them under a dedicated
// prefix so a periodic janitor can sweep abandoned uploads.
const StagingPrefix = "vod-staging/"

// ContentPrefix is the prefix for the final, content-addressed object.
const ContentPrefix = "vod/"

// ProcessVOD runs the streaming pipeline for one VOD upload and returns
// the BDASL CID of the resulting fMP4. The output object is written at
// ContentPrefix+<cid>.fmp4 in the configured S3 bucket; staging objects
// are cleaned up on success or failure.
func ProcessVOD(ctx context.Context, cli *config.CLI, in Input) (string, error) {
	ctx = log.WithLogValues(ctx, "func", "ProcessVOD", "uploadId", in.UploadID, "did", in.RepoDID)
	log.Log(ctx, "starting VOD processing", "backend", in.Backend, "size", in.Size, "mimeType", in.MimeType)

	if !cli.S3Configured() {
		return "", errors.New("vod processing requires S3 to be configured")
	}
	s3client := newS3Client(cli)

	src, size, closer, err := openSource(ctx, cli, s3client, in)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}
	defer closer()

	stagingKey := StagingPrefix + in.UploadID + ".fmp4"
	staging, err := s3pkg.NewMultipartWriter(ctx, s3client, cli.S3Bucket, stagingKey, "video/mp4")
	if err != nil {
		return "", fmt.Errorf("start staging upload: %w", err)
	}
	defer staging.Close() // Abort is idempotent; no-op once Complete ran

	hasher := bdasl.NewWriter()
	final := io.MultiWriter(hasher, staging)

	cid, err := streamThroughMuxl(ctx, src, size, final)
	if err != nil {
		// staging is aborted by the deferred Close
		return "", err
	}
	_ = cid // computed by hasher; kept here to match the variable name in the loop

	if err := staging.Complete(); err != nil {
		return "", fmt.Errorf("complete staging upload: %w", err)
	}

	finalCID := hasher.CID()
	contentKey := ContentPrefix + finalCID + ".fmp4"

	if err := finalizeUpload(ctx, s3client, cli.S3Bucket, stagingKey, contentKey); err != nil {
		return "", fmt.Errorf("finalize: %w", err)
	}

	log.Log(ctx, "VOD processed", "cid", finalCID, "key", contentKey)
	return finalCID, nil
}

// streamThroughMuxl wires up the goroutines that connect:
//
//	[gstreamer pipeline] -> mp4mux output (io.Pipe)
//	                                      -> muxl concatenator -> dst
//
// gstreamer writes the fMP4 stream to one end of an io.Pipe; a goroutine
// forwards those bytes into the muxl concatenator; another goroutine
// drains the concatenator's init+seg channels and writes them to dst.
// Returns the unused first return value as a placeholder for an
// eventual hashing/CID computation that's actually performed by the
// caller via the TeeReader/MultiWriter wired into dst.
func streamThroughMuxl(ctx context.Context, src io.ReaderAt, size int64, dst io.Writer) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pr, pw := io.Pipe()

	// Forwarder: gstreamer mp4mux output -> muxl concatenator stdin.
	// Run as a goroutine because both ends are pipes; both sides need to
	// be live for either to make progress.
	concat := muxl.NewConcatenator(ctx)
	feedDone := make(chan error, 1)
	go func() {
		defer close(feedDone)
		buf := make([]byte, 64*1024)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				if werr := concat.Write(buf[:n]); werr != nil {
					feedDone <- werr
					return
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					feedDone <- concat.Close()
				} else {
					feedDone <- err
				}
				return
			}
		}
	}()

	// Consumer: muxl concatenator output channels -> dst.
	consumeDone := make(chan error, 1)
	go func() { consumeDone <- consumeConcat(concat, dst) }()

	// Run gstreamer on this goroutine; mp4mux output bytes land at pw.
	pipelineErr := media.RunVODPipeline(ctx, src, size, pw)
	// Close pw so the forwarder sees EOF and can close the concat.
	_ = pw.Close()

	// Wait for both downstream goroutines. We collect any error but
	// prefer surfacing the pipeline error since downstream errors are
	// often a consequence of it.
	feedErr := <-feedDone
	consumeErr := <-consumeDone

	if pipelineErr != nil {
		return "", fmt.Errorf("gstreamer pipeline: %w", pipelineErr)
	}
	if feedErr != nil {
		return "", fmt.Errorf("muxl feed: %w", feedErr)
	}
	if consumeErr != nil {
		return "", fmt.Errorf("muxl consume: %w", consumeErr)
	}
	return "", nil
}

// consumeConcat drains the concatenator's two output channels and writes
// their contents (init, then segments) to dst in order of arrival. If
// the init segment changes mid-stream (multi-input concatenation), the
// new init is written too — for VOD with a single input that doesn't
// happen, but the loop handles it for free.
func consumeConcat(c *muxl.Concatenator, dst io.Writer) error {
	initCh, segCh := c.InitCh, c.SegCh
	for initCh != nil || segCh != nil {
		select {
		case init, ok := <-initCh:
			if !ok {
				initCh = nil
				continue
			}
			if _, err := dst.Write(init); err != nil {
				return err
			}
		case seg, ok := <-segCh:
			if !ok {
				segCh = nil
				continue
			}
			if _, err := dst.Write(seg); err != nil {
				return err
			}
		}
	}
	return nil
}

// openSource returns a ReaderAt + size for the upload, regardless of
// backend. The returned closer must be called when the caller is done
// with the source.
func openSource(ctx context.Context, cli *config.CLI, s3client *awss3.Client, in Input) (io.ReaderAt, int64, func(), error) {
	switch in.Backend {
	case BackendFile:
		f, err := os.Open(in.Location)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("open file upload %q: %w", in.Location, err)
		}
		st, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return nil, 0, nil, fmt.Errorf("stat file upload %q: %w", in.Location, err)
		}
		return f, st.Size(), func() { _ = f.Close() }, nil
	case BackendS3:
		bucket, key, err := s3pkg.ParseURL(in.Location)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("parse s3 location %q: %w", in.Location, err)
		}
		ra, err := s3pkg.NewReaderAt(ctx, s3client, bucket, key)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("open s3 upload: %w", err)
		}
		return ra, ra.Size(), func() { _ = ra.Close() }, nil
	default:
		return nil, 0, nil, fmt.Errorf("unknown upload backend %q", in.Backend)
	}
}

// finalizeUpload renames the staging object to its content-addressed
// key. S3 has no rename: we CopyObject server-side, then DeleteObject
// the staging key. If the content key already exists (duplicate upload
// of identical content), we still proceed — copy is idempotent and we
// still want to drop the staging copy.
func finalizeUpload(ctx context.Context, c *awss3.Client, bucket, stagingKey, contentKey string) error {
	_, err := c.CopyObject(ctx, &awss3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(contentKey),
		CopySource: aws.String(bucket + "/" + stagingKey),
	})
	if err != nil {
		return fmt.Errorf("copy staging -> %s: %w", contentKey, err)
	}
	if _, err := c.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(stagingKey),
	}); err != nil {
		// Non-fatal: the content key is in place; staging will be swept
		// later. Log + continue.
		log.Warn(ctx, "failed to delete staging object", "key", stagingKey, "error", err)
	}
	return nil
}

func newS3Client(cli *config.CLI) *awss3.Client {
	return awss3.New(awss3.Options{
		Region: cli.S3Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cli.S3AccessKeyID,
			cli.S3SecretAccessKey,
			"",
		),
		BaseEndpoint: aws.String(cli.S3Endpoint),
		UsePathStyle: true,
	})
}
