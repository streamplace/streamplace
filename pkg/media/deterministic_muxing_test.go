package media

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/test/remote"
)

var muxTestCount = 2

func TestDeterministicMuxing(t *testing.T) {
	withNoGSTLeaks(t, func() {
		tempDir, err := os.MkdirTemp("", "deterministic_muxing_test")
		require.NoError(t, err)

		startFile := remote.RemoteArchive("14ba49843a56c0510e2b5059123abd2f98a502b1f4c7d706b0ae1066d438468c/BigBuckBunny_1sGOP_4kp60_NoBframes.1min.tar.gz")

		require.NoError(t, err)
		for i := 0; i < muxTestCount; i++ {
			splitAndCombineTest(t, tempDir, startFile)
			require.NoError(t, err)
		}
	})
}

func splitAndCombineTest(t *testing.T, tempDir string, inputDir string) string {
	var err error
	tempDir, err = os.MkdirTemp(tempDir, "splitAndCombineTest")
	require.NoError(t, err)

	firstReport, err := makeSegDirReport(t, inputDir)
	require.NoError(t, err)

	combinedHashes := []string{}
	combinedFiles := []string{}
	for i := 0; i < muxTestCount; i++ {
		outFilePath := filepath.Join(tempDir, fmt.Sprintf("combined_%d.mp4", i))
		combinedFiles = append(combinedFiles, outFilePath)
		outFile, err := os.Create(outFilePath)
		log.Log(context.Background(), "creating combined file", "file", outFilePath)
		require.NoError(t, err)
		defer outFile.Close()
		err = Clip(context.Background(), firstReport.Segs, outFile)
		require.NoError(t, err)
		hash, err := hashFile(outFilePath)
		require.NoError(t, err)
		combinedHashes = append(combinedHashes, hash)
	}

	for _, hash := range combinedHashes {
		require.Equal(t, hash, combinedHashes[0])
	}

	for i := 0; i < muxTestCount; i++ {
		segDir, err := os.MkdirTemp(tempDir, "segs")
		require.NoError(t, err)
		err = SegmentFile(context.Background(), combinedFiles[0], segDir)
		require.NoError(t, err)
		report, err := makeSegDirReport(t, segDir)
		require.NoError(t, err)
		require.NoError(t, report.CheckEquals(firstReport), "round-trip muxing is not deterministic")
	}

	return combinedFiles[0]
}

type SegDirReport struct {
	Dir    string
	Segs   []string
	Hashes []string
}

func makeSegDirReport(t *testing.T, segDir string) (*SegDirReport, error) {
	segs := []string{}
	segEntries, err := os.ReadDir(segDir)
	require.NoError(t, err)
	for _, segEntry := range segEntries {
		if segEntry.Type().IsRegular() {
			segPath := filepath.Join(segDir, segEntry.Name())
			segs = append(segs, segPath)
		}
	}
	sort.Strings(segs)
	hashes := make([]string, len(segs))
	for i, segPath := range segs {
		hash, err := hashFile(segPath)
		if err != nil {
			return nil, err
		}
		hashes[i] = hash
	}

	return &SegDirReport{
		Dir:    segDir,
		Segs:   segs,
		Hashes: hashes,
	}, nil
}

func (s *SegDirReport) Equals(other *SegDirReport) bool {
	if len(s.Segs) != len(other.Segs) {
		return false
	}
	if len(s.Hashes) != len(other.Hashes) {
		return false
	}
	return reflect.DeepEqual(s.Hashes, other.Hashes)
}

func hashFile(path string) (string, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(bs)
	return fmt.Sprintf("%x", hash), nil
}

func (s *SegDirReport) CheckEquals(other *SegDirReport) error {
	if !s.Equals(other) {
		str1 := s.ToString()
		str2 := other.ToString()
		return fmt.Errorf("files should be equal: %s\n%s", str1, str2)
	}
	return nil
}

func (s *SegDirReport) ToString() string {
	bs, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bs)
}
