package vod

import "context"

// ServableVideo is what a playback transport needs to serve one viewer a
// video's segments: the content blob, and each track's segments as absolute
// byte ranges into it, in metafile order.
type ServableVideo struct {
	OwnerDID string
	SID      string
	BlobCID  string // content blob: BlobsPrefix + BlobCID + ".mp4"
	Tracks   map[string][]ByteRange
}

type ByteRange struct {
	Offset, Size int64
}

// VideoResolver resolves the video at at://did/place.stream.video/rkey for a
// playback session, gated like HTTP playback: an error means refuse it.
type VideoResolver interface {
	ResolveServableVideo(ctx context.Context, did, rkey, sid string) (*ServableVideo, error)
}
