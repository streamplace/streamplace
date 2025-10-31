package media

import (
	"bytes"
	"context"
	"os"
)

// CombineSegments combines a list of segments into a single segment that maintains all of the manifests
func CombineSegments(ctx context.Context, sources []string, ms MediaSigner) ([]byte, error) {
	buf := bytes.Buffer{}
	err := CombineSegmentsUnsigned(ctx, sources, &buf)
	if err != nil {
		return nil, err
	}
	ingredients := [][]byte{}
	for _, source := range sources {
		bs, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		ingredients = append(ingredients, bs)
	}
	signedConcatBS, err := ms.SignConcatMP4(context.Background(), bytes.NewReader(buf.Bytes()), ingredients)
	if err != nil {
		return nil, err
	}
	return signedConcatBS, nil
}
