package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// localCollections are the collections a node keeps for their authors
// instead of their repos, each with a check that the node can act on a
// record: an encoder told to use a codec it doesn't have fails to stream.
var localCollections = map[string]func(glex.Record) error{
	"place.stream.ingest.videoRendition": func(rec glex.Record) error {
		r := rec.(*placestream.IngestVideoRendition)
		if r.Codec != "h264" {
			return fmt.Errorf("unsupported video codec %q", r.Codec)
		}
		if r.Width < 1 || r.Height < 1 || r.Bitrate < 1 || r.Framerate.Numerator < 1 || r.Framerate.Denominator < 1 {
			return fmt.Errorf("width, height, bitrate and framerate must be positive")
		}
		return nil
	},
	"place.stream.ingest.audioRendition": func(rec glex.Record) error {
		r := rec.(*placestream.IngestAudioRendition)
		if !slices.Contains([]string{"aac", "opus"}, r.Codec) {
			return fmt.Errorf("unsupported audio codec %q", r.Codec)
		}
		if r.Bitrate < 1 {
			return fmt.Errorf("bitrate must be positive")
		}
		return nil
	},
}

// localRecordCaller checks that the caller is signed in as repo and that
// collection is one this node keeps locally.
func localRecordCaller(ctx context.Context, repo, collection string) error {
	session := GetCaller(ctx)
	if session == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	if session.DID != repo {
		return echo.NewHTTPError(http.StatusForbidden, "local records can only be kept for your own repo")
	}
	if _, ok := localCollections[collection]; !ok {
		return &xrpc.Error{
			StatusCode: http.StatusBadRequest,
			Wrapped:    &xrpc.XRPCError{ErrStr: "InvalidCollection", Message: fmt.Sprintf("this node does not keep %s records locally", collection)},
		}
	}
	return nil
}

func (s *Server) handlePlaceStreamLocalPutRecord(ctx context.Context, input *placestream.LocalPutRecord_Input) (*placestream.LocalPutRecord_Output, error) {
	if err := localRecordCaller(ctx, input.Repo, input.Collection); err != nil {
		return nil, err
	}
	if _, err := syntax.ParseRecordKey(input.Rkey); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid rkey: %s", err))
	}
	if input.Record == nil || input.Record.Val == nil || input.Record.Val.RecordTypeID() != input.Collection {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("record must be a %s", input.Collection))
	}
	if err := localCollections[input.Collection](input.Record.Val); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	record, err := input.Record.MarshalCBOR()
	if err != nil {
		return nil, fmt.Errorf("encoding record: %w", err)
	}
	cid, err := spid.GetCIDFromBytes(record)
	if err != nil {
		return nil, fmt.Errorf("computing record cid: %w", err)
	}
	row := &localdb.LocalRecord{RepoDID: input.Repo, Collection: input.Collection, Rkey: input.Rkey, CID: cid.String(), Record: record}
	if err := s.localDB.PutLocalRecord(row); err != nil {
		return nil, err
	}
	return &placestream.LocalPutRecord_Output{Uri: row.URI(), Cid: row.CID}, nil
}

func (s *Server) handlePlaceStreamLocalListRecords(ctx context.Context, collection string, repo string) (*placestream.LocalListRecords_Output, error) {
	if err := localRecordCaller(ctx, repo, collection); err != nil {
		return nil, err
	}
	rows, err := s.localDB.ListLocalRecords(repo, collection)
	if err != nil {
		return nil, err
	}
	out := &placestream.LocalListRecords_Output{Records: []placestream.LocalListRecords_Record{}}
	for _, row := range rows {
		value := &glex.LexiconTypeDecoder{}
		if err := value.UnmarshalCBOR(row.Record); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", row.URI(), err)
		}
		out.Records = append(out.Records, placestream.LocalListRecords_Record{Uri: row.URI(), Cid: row.CID, Value: value})
	}
	return out, nil
}

func (s *Server) handlePlaceStreamLocalDeleteRecord(ctx context.Context, input *placestream.LocalDeleteRecord_Input) (*placestream.LocalDeleteRecord_Output, error) {
	if err := localRecordCaller(ctx, input.Repo, input.Collection); err != nil {
		return nil, err
	}
	if err := s.localDB.DeleteLocalRecord(input.Repo, input.Collection, input.Rkey); err != nil {
		return nil, err
	}
	return &placestream.LocalDeleteRecord_Output{}, nil
}
