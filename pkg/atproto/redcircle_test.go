package atproto

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateRedCircle(t *testing.T) {
	bs, err := os.ReadFile("robot.jpg")
	require.NoError(t, err)
	img, err := GenerateRedCircle(context.Background(), bs)
	require.NoError(t, err)
	require.NotNil(t, img)
	f, err := os.Create("test.jpg")
	require.NoError(t, err)
	defer f.Close()
	_, err = f.Write(img)
	require.NoError(t, err)
}
