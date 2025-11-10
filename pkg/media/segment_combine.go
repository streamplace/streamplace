package media

import (
	"bytes"
	"context"
	"io"
	"os"

	"stream.place/streamplace/pkg/aqio"
)

// CombineSegments combines a list of segments into a single segment that maintains all of the manifests
func CombineSegments(ctx context.Context, sources []string, ms MediaSigner) ([]byte, error) {
	inputFds := make([]io.ReadSeeker, len(sources))
	for i, source := range sources {
		fd, err := os.Open(source)
		if err != nil {
			return nil, err
		}
		inputFds[i] = fd
		defer fd.Close()
	}
	rws := aqio.NewReadWriteSeeker([]byte{})
	err := CombineSegmentsUnsigned(ctx, inputFds, rws)
	if err != nil {
		return nil, err
	}
	// rewind all the inputs for the signer
	for _, fd := range inputFds {
		_, err := fd.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}
	bs, err := rws.Bytes()
	if err != nil {
		return nil, err
	}
	signedConcatBS, err := ms.SignConcatMP4(context.Background(), bytes.NewReader(bs), inputFds)
	if err != nil {
		return nil, err
	}
	return signedConcatBS, nil
}
