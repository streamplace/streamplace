package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/repo"
	"github.com/ipfs/go-cid"
	"github.com/labstack/echo/v4"
	"github.com/multiformats/go-multihash"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamLiveGetSegments(ctx context.Context, before string, limit int, userDID string) (*placestreamtypes.LiveGetSegments_Output, error) {
	if userDID == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "User DID is required")
	}
	var beforeTime *time.Time
	if before != "" {
		parsedTime, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid 'before' parameter: must be RFC3339 format")
		}
		beforeTime = &parsedTime
	}

	segments, err := s.model.LatestSegmentsForUser(userDID, limit, beforeTime)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch segments")
	}

	// Convert segments to the expected output format
	output := &placestreamtypes.LiveGetSegments_Output{
		Segments: make([]*placestreamtypes.Segment_SegmentView, len(segments)),
	}

	for i, segment := range segments {
		record, err := segment.ToStreamplaceSegment()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to convert segment to streamplace segment: %s", err))
		}
		c, err := getCID(record)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get CID: %s", err))
		}
		ltd := &util.LexiconTypeDecoder{Val: record}

		output.Segments[i] = &placestreamtypes.Segment_SegmentView{
			Record: ltd,
			Cid:    c.String(),
		}
	}

	return output, nil
}

func getCID(record repo.CborMarshaler) (*cid.Cid, error) {
	builder := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256)
	buf := bytes.NewBuffer(nil)
	err := record.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	c, err := builder.Sum(buf.Bytes())
	if err != nil {
		return nil, err
	}
	return &c, nil
}
