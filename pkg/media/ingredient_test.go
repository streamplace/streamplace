package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	ct "stream.place/streamplace/pkg/config/configtesting"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/test/remote"
)

var testTimestamp = "2025-01-01T00:00:00.000Z"

func TestIngredientConcat(t *testing.T) {
	withNoGSTLeaks(t, func() {
		tempDir, err := os.MkdirTemp("", "ingredient_test")
		require.NoError(t, err)
		// defer os.RemoveAll(tempDir)
		segments := remote.RemoteArchive("14ba49843a56c0510e2b5059123abd2f98a502b1f4c7d706b0ae1066d438468c/BigBuckBunny_1sGOP_4kp60_NoBframes.1min.tar.gz")
		// segments := "/Users/iameli/testvids/three"
		testVids := []string{}
		segEntries, err := os.ReadDir(segments)
		require.NoError(t, err)
		for _, segEntry := range segEntries {
			if segEntry.Type().IsRegular() {
				testVids = append(testVids, filepath.Join(segments, segEntry.Name()))
			}
		}
		firstReport, err := makeSegDirReport(t, segments)
		require.NoError(t, err)
		priv, _, err := spkey.GenerateStreamKey()
		require.NoError(t, err)
		signer, err := spkey.KeyToSigner(priv)
		require.NoError(t, err)
		cli := ct.CLI(t, &config.CLI{
			TAURL:    "http://timestamp.digicert.com",
			WideOpen: true,
		})
		msInterface, err := MakeMediaSigner(context.Background(), cli, "test-person", signer, nil)
		require.NoError(t, err)
		ms := msInterface.(*MediaSignerLocal)
		buf := bytes.Buffer{}
		err = CombineSegmentsUnsigned(context.Background(), testVids, &buf)
		require.NoError(t, err)
		ingredients := [][]byte{}
		startTS, err := time.Parse(util.ISO8601, testTimestamp)
		require.NoError(t, err)
		signedSegDir := makeTestSubdir(t, tempDir, "signed-segments")
		for i, vid := range testVids {
			ts := startTS.Add(time.Duration(i) * time.Second)
			bs, err := os.ReadFile(vid)
			require.NoError(t, err)
			signedBS, err := ms.SignMP4(context.Background(), bytes.NewReader(bs), ts.UnixMilli())
			require.NoError(t, err)
			ingredients = append(ingredients, signedBS)
			err = os.WriteFile(filepath.Join(signedSegDir, fmt.Sprintf("signed_%06d.mp4", i)), signedBS, 0644)
			require.NoError(t, err)
		}
		signedReport, err := makeSegDirReport(t, signedSegDir)
		require.NoError(t, err)
		signedConcatBS, err := ms.SignConcatMP4(context.Background(), bytes.NewReader(buf.Bytes()), ingredients)
		require.NoError(t, err)
		require.Greater(t, len(signedConcatBS), 0)
		concatSegment := filepath.Join(tempDir, "ingredient-concat.mp4")
		err = os.WriteFile(concatSegment, signedConcatBS, 0644)
		require.NoError(t, err)
		splitSegs, err := SegmentFileUnsigned(context.Background(), concatSegment)
		require.NoError(t, err)
		splitSegDir := makeTestSubdir(t, tempDir, "split-segments")
		for i, unsignedSeg := range splitSegs {
			err = os.WriteFile(filepath.Join(splitSegDir, fmt.Sprintf("unsigned_%06d.mp4", i)), unsignedSeg, 0644)
			require.NoError(t, err)
		}
		splitReport, err := makeSegDirReport(t, splitSegDir)
		require.NoError(t, err)
		require.NoError(t, splitReport.CheckEquals(firstReport), "split segments are not equal to original segments")
		signedSplitSegDir := makeTestSubdir(t, tempDir, "signed-split-segments")
		resignedSegs, err := SplitSegments(context.Background(), signedConcatBS)
		require.NoError(t, err)
		for _, resignedSeg := range resignedSegs {
			err = os.WriteFile(filepath.Join(signedSplitSegDir, resignedSeg.Filename), resignedSeg.Data, 0644)
			require.NoError(t, err)
		}
		signedSplitReport, err := makeSegDirReport(t, signedSplitSegDir)
		require.NoError(t, err)
		require.NoError(t, signedSplitReport.CheckEquals(signedReport), "split signed segments are not equal to signed segments")
	})
}

func makeTestSubdir(t *testing.T, tempDir, subdir string) string {
	subDir := filepath.Join(tempDir, subdir)
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)
	return subDir
}
