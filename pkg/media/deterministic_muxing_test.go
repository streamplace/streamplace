package media

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/test/remote"
)

var muxTestCount = 2

func TestDeterministicMuxing(t *testing.T) {
	withNoGSTLeaks(t, func() {
		tempDir, err := os.MkdirTemp("", "deterministic_muxing_test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		startFile := remote.RemoteFixture("7629bc0e2b3964743bfee06cc59dceabc5b4637b19de8e3fea0889cb421782d8/perfect.mp4")
		for i := 0; i < muxTestCount; i++ {
			startFile = splitAndCombineTest(t, tempDir, startFile)
		}
	})
}

func splitAndCombineTest(t *testing.T, tempDir string, inputPath string) string {
	var err error
	tempDir, err = os.MkdirTemp(tempDir, "splitAndCombineTest")
	require.NoError(t, err)

	segDirs := []string{}
	for i := 0; i < muxTestCount; i++ {
		segDir, err := os.MkdirTemp(tempDir, "segs")
		segDirs = append(segDirs, segDir)
		require.NoError(t, err)
		err = SegmentFile(context.Background(), inputPath, segDir)
		require.NoError(t, err)
	}
	firstReport, err := makeSegDirReport(t, segDirs[0])
	require.NoError(t, err)
	for _, segDir := range segDirs[1:] {
		report, err := makeSegDirReport(t, segDir)
		require.NoError(t, err)
		require.True(t, report.Equals(firstReport))
	}

	combinedHashes := []string{}
	combinedFiles := []string{}
	for i := 0; i < muxTestCount; i++ {
		outFilePath := filepath.Join(tempDir, fmt.Sprintf("combined_%d.mp4", i))
		combinedFiles = append(combinedFiles, outFilePath)
		outFile, err := os.Create(outFilePath)
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
		hashes[i] = fmt.Sprintf("%x", hash)
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
