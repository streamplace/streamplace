package atproto

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateRedCircle(t *testing.T) {
	testCases := []struct {
		infile  string
		outfile string
	}{
		{"robot.jpg", "test.jpg"},
		{"scumbag.png", "test_scumbag.jpg"},
	}

	for _, tc := range testCases {
		tc := tc // capture for parallel tests
		t.Run(tc.infile, func(t *testing.T) {
			bs, err := os.ReadFile(tc.infile)
			require.NoError(t, err)
			img, err := GenerateRedCircle(context.Background(), bs)
			require.NoError(t, err)
			require.NotNil(t, img)
			f, err := os.Create(tc.outfile)
			require.NoError(t, err)
			defer f.Close()
			_, err = f.Write(img)
			require.NoError(t, err)
		})
	}
}
