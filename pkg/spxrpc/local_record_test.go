package spxrpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/placestream"
)

// A node keeps its local collections for the signed-in author only, refuses
// records it couldn't act on, and hands back what it kept with its $type.
func TestLocalRecords(t *testing.T) {
	ldb, err := localdb.MakeDB(":memory:")
	require.NoError(t, err)
	s := &Server{localDB: ldb}
	const me = "did:plc:me"
	ctx := context.WithValue(context.Background(), pdsAuthContextKey, me)
	const video = "place.stream.ingest.videoRendition"
	put := func(body string) error {
		var input placestream.LocalPutRecord_Input
		require.NoError(t, json.Unmarshal([]byte(body), &input))
		_, err := s.handlePlaceStreamLocalPutRecord(ctx, &input)
		return err
	}
	record := func(codec string) string {
		return `{"$type":"` + video + `","codec":"` + codec + `","width":1920,"height":1080,"framerate":{"numerator":60,"denominator":1},"bitrate":6000,"createdAt":"2026-10-18T00:00:00Z"}`
	}

	require.NoError(t, put(`{"repo":"`+me+`","collection":"`+video+`","rkey":"main","record":`+record("av1")+`}`))
	require.NoError(t, put(`{"repo":"`+me+`","collection":"`+video+`","rkey":"main","record":`+record("h264")+`}`))
	require.ErrorContains(t, put(`{"repo":"did:plc:other","collection":"`+video+`","rkey":"main","record":`+record("h264")+`}`), "your own repo")
	require.ErrorContains(t, put(`{"repo":"`+me+`","collection":"place.stream.livestream","rkey":"main","record":`+record("h264")+`}`), "InvalidCollection")
	require.ErrorContains(t, put(`{"repo":"`+me+`","collection":"place.stream.ingest.audioRendition","rkey":"main","record":`+record("h264")+`}`), "must be a place.stream.ingest.audioRendition")
	require.ErrorContains(t, put(`{"repo":"`+me+`","collection":"`+video+`","rkey":"main","record":`+record("vp8")+`}`), "unsupported video codec")

	list, err := s.handlePlaceStreamLocalListRecords(ctx, video, me)
	require.NoError(t, err)
	require.Len(t, list.Records, 1)
	require.Equal(t, "at://"+me+"/"+video+"/main", list.Records[0].Uri)
	got, err := json.Marshal(list.Records[0].Value)
	require.NoError(t, err)
	require.JSONEq(t, record("h264"), string(got), "the last record written, h264 kept over the refused vp8")

	_, err = s.handlePlaceStreamLocalDeleteRecord(ctx, &placestream.LocalDeleteRecord_Input{Repo: me, Collection: video, Rkey: "main"})
	require.NoError(t, err)
	list, err = s.handlePlaceStreamLocalListRecords(ctx, video, me)
	require.NoError(t, err)
	require.Empty(t, list.Records)
}
