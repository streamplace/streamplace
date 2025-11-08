package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqio"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/log"
)

type SplitSegment struct {
	Filename string
	Data     []byte
}

type ManifestResult struct {
	Manifests map[string]c2patypes.Manifest `json:"manifests"`
	Certs     map[string]string             `json:"certs"`
}

type ManifestAndMetadata struct {
	Manifest        c2patypes.Manifest
	SegmentMetadata *SegmentMetadata
}

type ReadWriteSeekCloser interface {
	io.ReadWriteSeeker
	io.Closer
}

type SegmentToSign struct {
	unsignedSeg io.ReadSeeker
	manifestId  string
	cert        []byte
	outputSeg   io.ReadWriteSeeker
	closeCh     chan struct{}
}

func NewSegmentToSign(unsignedSeg io.ReadSeeker, manifestId string, cert []byte, outputSeg io.ReadWriteSeeker) *SegmentToSign {
	return &SegmentToSign{
		unsignedSeg: unsignedSeg,
		manifestId:  manifestId,
		cert:        cert,
		outputSeg:   outputSeg,
		closeCh:     make(chan struct{}),
	}
}

func (s *SegmentToSign) UnsignedSegStream() iroh_streamplace.Stream {
	return c2patypes.NewReader(s.unsignedSeg)
}

func (s *SegmentToSign) ManifestId() string {
	return s.manifestId
}

func (s *SegmentToSign) Cert() []byte {
	return s.cert
}

func (s *SegmentToSign) OutputSegStream() iroh_streamplace.Stream {
	return c2patypes.NewWriter(s.outputSeg)
}

func (s *SegmentToSign) Close() {
	close(s.closeCh)
}

func (s *SegmentToSign) Done() {
	<-s.closeCh
}

type ManySegmentsToSign struct {
	segmentCh chan iroh_streamplace.SegmentToSign
	readyCh   chan struct{}
}

func (m *ManySegmentsToSign) Next() *iroh_streamplace.SegmentToSign {
	if m.readyCh != nil {
		close(m.readyCh)
		m.readyCh = nil
	}
	seg := <-m.segmentCh
	if seg == nil {
		return nil
	}
	return &seg
}

// split a signed concatenated mp4 into its constituent signed segments
func SplitSegments(ctx context.Context, input io.ReadSeeker, cb func(fname string) ReadWriteSeekCloser) error {
	manifestsStr, err := iroh_streamplace.GetManifests(c2patypes.NewReader(input))
	if err != nil {
		return fmt.Errorf("failed to get manifests: %w", err)
	}
	_, err = input.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}
	var manifests ManifestResult
	err = json.Unmarshal([]byte(manifestsStr), &manifests)
	if err != nil {
		return fmt.Errorf("failed to unmarshal manifests: %w", err)
	}
	manifestList := []ManifestAndMetadata{}
	for _, manifest := range manifests.Manifests {
		metadata, err := ParseSegmentAssertions(context.Background(), &manifest)
		if errors.Is(err, ErrMissingMetadata) {
			log.Error(ctx, "missing metadata", "manifest", manifest.Label)
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to parse segment assertions: %w", err)
		}
		manifestList = append(manifestList, ManifestAndMetadata{
			Manifest:        manifest,
			SegmentMetadata: metadata,
		})
	}
	sort.Slice(manifestList, func(i, j int) bool {
		m1 := manifestList[i]
		m2 := manifestList[j]
		return m1.SegmentMetadata.StartTime.Time().Before(m2.SegmentMetadata.StartTime.Time())
	})
	certList := [][]byte{}
	for _, manifest := range manifestList {
		certList = append(certList, []byte(manifests.Certs[*manifest.Manifest.Label]))
	}

	segmentCh := make(chan iroh_streamplace.SegmentToSign)
	readyCh := make(chan struct{})
	mss := &ManySegmentsToSign{
		segmentCh: segmentCh,
		readyCh:   readyCh,
	}
	g, ctx := errgroup.WithContext(ctx)
	unsignedCh := make(chan *SplitSegment)

	// note: we're passing the input to two places here and need to make sure
	// they're not running into problems with concurrent seeking. so we use
	// this readyCh as a hack - it only fires after Rust is done with the input

	g.Go(func() error {
		err := iroh_streamplace.Resign(mss, c2patypes.NewReader(input))
		if err != nil {
			return fmt.Errorf("failed to resign segments: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		defer close(unsignedCh)
		<-readyCh
		// rust is done with the input, rewind and start segmenting
		_, err := input.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek to start: %w", err)
		}
		err = SegmentUnsigned(ctx, input, unsignedCh)
		if err != nil {
			return fmt.Errorf("failed to segment file: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		defer close(segmentCh)
		i := 0
		for unsignedSeg := range unsignedCh {
			meta := manifestList[i].SegmentMetadata
			fname := fmt.Sprintf("%s.mp4", meta.StartTime.FileSafeString())
			rwsc := cb(fname)
			rws := aqio.NewReadWriteSeeker(unsignedSeg.Data)
			ss := NewSegmentToSign(c2patypes.NewReader(rws), *manifestList[i].Manifest.Label, certList[i], rwsc)
			i += 1
			segmentCh <- ss
			ss.Done()
			err := rwsc.Close()
			if err != nil {
				return fmt.Errorf("failed to close segment file: %w", err)
			}
		}
		return nil
	})

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("failed to split segments: %w", err)
	}

	// err = iroh_streamplace.Resign(inputStreams, c2patypes.NewReader(aqio.NewReadWriteSeeker(input)), manifestStrs, certList, outputStreams)
	// if err != nil {
	// 	fmt.Errorf("failed to resign segments: %w", err)
	// }
	// splitSegments := []SplitSegment{}
	// for i, resignedSeg := range outputStreams.Streams {
	// 	aqrws := resignedSeg.(*aqio.ReadWriteSeeker)
	// 	data, err := aqrws.Bytes()
	// 	if err != nil {
	// 		fmt.Errorf("failed to read resigned segment: %w", err)
	// 	}
	// 	meta := manifestList[i].SegmentMetadata
	// 	fname := fmt.Sprintf("%s.mp4", meta.StartTime.FileSafeString())
	// 	splitSegments = append(splitSegments, SplitSegment{
	// 		Filename: fname,
	// 		Data:     data,
	// 	})
	// }
	return nil
}
