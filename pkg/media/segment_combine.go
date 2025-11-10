package media

import (
	"bytes"
	"context"
	"io"

	"stream.place/streamplace/pkg/aqio"
)

// CombineSegments combines a list of segments into a single segment that maintains all of the manifests
func CombineSegments(ctx context.Context, inputFds []io.ReadSeeker, ms MediaSigner, output io.ReadWriteSeeker) error {
	rws := aqio.NewReadWriteSeeker([]byte{})
	err := CombineSegmentsUnsigned(ctx, inputFds, rws)
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
	bs, err := rws.Bytes()
	if err != nil {
		return err
	}
	err = ms.SignConcatMP4(context.Background(), bytes.NewReader(bs), inputFds, output)
	if err != nil {
		return err
	}
	return nil
}
