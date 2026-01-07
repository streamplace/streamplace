package media

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqio"
	"stream.place/streamplace/pkg/config"
	ct "stream.place/streamplace/pkg/config/configtesting"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/test/remote"
)

var testTimestamp = "2025-01-01T00:00:00.000Z"

func makeServerMediaSigner(t *testing.T) *MediaSignerLocal {
	priv, _, err := spkey.GenerateStreamKey()
	require.NoError(t, err)
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
	return ms
}

func TestSegmentRoundtrip(t *testing.T) {
	testCases := []struct {
		name    string
		fixture string
	}{
		// {
		// 	name:    "OneMinute",
		// 	fixture: remote.RemoteArchive("4563c7b48c0ca02c3fc87bbe6f1e63a743656e465a82bec0af75ef7eead04a23/1-minute-of-signed-segments.tar.gz"),
		// },
		{
			name:    "ThreeSegs",
			fixture: remote.RemoteArchive("c21e9352e72ca0729c66af2fcabec1b8997b509601241e8d38d5728f9687386b/threesegs.tar.gz"),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			withNoGSTLeaks(t, func() {
				tempDir, err := os.MkdirTemp("", "ingredient_test")
				t.Logf("tempDir: %s", tempDir)
				require.NoError(t, err)
				getTestVids := func() []io.ReadSeeker {
					testVids := []io.ReadSeeker{}
					segEntries, err := os.ReadDir(testCase.fixture)
					require.NoError(t, err)
					for _, segEntry := range segEntries {
						if segEntry.Type().IsRegular() {
							if !strings.HasSuffix(segEntry.Name(), ".mp4") {
								continue
							}
							fd, err := os.Open(filepath.Join(testCase.fixture, segEntry.Name()))
							require.NoError(t, err)
							testVids = append(testVids, fd)
						}
					}
					return testVids
				}

				firstReport, err := makeSegDirReport(t, testCase.fixture)
				require.NoError(t, err)
				ms := makeServerMediaSigner(t)
				rws := aqio.NewReadWriteSeeker([]byte{})
				err = CombineSegments(context.Background(), getTestVids(), ms, rws)
				require.NoError(t, err)

				_, err = rws.Seek(0, io.SeekStart)
				require.NoError(t, err)

				signedSplitSegDir := makeTestSubdir(t, tempDir, "signed-split-segments")
				cli := &config.CLI{}
				fs := cli.NewFlagSet("rtcrec-test")
				err = cli.Parse(fs, []string{})
				require.NoError(t, err)
				err = SplitSegments(context.Background(), cli, rws, func(fname string) ReadWriteSeekCloser {
					fd, err := os.Create(filepath.Join(signedSplitSegDir, fname))
					require.NoError(t, err)
					return fd
				})
				require.NoError(t, err)
				secondReport, err := makeSegDirReport(t, signedSplitSegDir)
				require.NoError(t, err)
				require.NoError(t, firstReport.CheckEquals(secondReport), "signed split segments are not equal to original segments")
			})
		})
	}
}

func makeTestSubdir(t *testing.T, tempDir, subdir string) string {
	subDir := filepath.Join(tempDir, subdir)
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)
	return subDir
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
