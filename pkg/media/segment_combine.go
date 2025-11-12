package media

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// CombineSegments combines a list of segments into a single segment that maintains all of the manifests
func CombineSegments(ctx context.Context, inputFds []io.ReadSeeker, ms MediaSigner, output io.ReadWriteSeeker) error {
	tempDir, err := os.MkdirTemp("", "streamplace-combine-segments")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	fd, err := os.Create(filepath.Join(tempDir, "combined-unsigned.mp4"))
	if err != nil {
		return err
	}
	defer fd.Close()
	err = CombineSegmentsUnsigned(ctx, inputFds, fd)
	if err != nil {
		return err
	}
	// rewind all the inputs for the signer
	for _, fd := range inputFds {
		_, err := fd.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
	}
	_, err = fd.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = ms.SignConcatMP4(context.Background(), fd, inputFds, output)
	if err != nil {
		return err
	}
	return nil
}
