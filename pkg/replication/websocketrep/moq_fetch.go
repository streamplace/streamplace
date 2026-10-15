package websocketrep

import (
	"bytes"
	"context"
	"fmt"
	"time"

	official "moq.dev/moq"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
)

func fetchMoqFrame(ctx context.Context, bc *official.BroadcastConsumer, track string, sequence uint64) (*official.Frame, error) {
	group, err := bc.FetchGroup(ctx, track, sequence, &official.FetchGroupOptions{Priority: 10})
	if err != nil {
		return nil, err
	}
	return readMoqGroup(ctx, group)
}

// FetchMoqGroup fetches exactly one origin-scoped group over a separate session.
// It validates and durably stores bytes without any live validation, bus event,
// source announcement, live window mutation, or recording lease acquisition.
// A cursor must never be moved to a different origin URL; rediscover its catalog.
func (r *WebsocketReplicator) FetchMoqGroup(ctx context.Context, rawURL, streamer, track string, sequence uint64) ([]byte, error) {
	if r.cli.DisableSyndication {
		return nil, fmt.Errorf("syndication disabled")
	}
	if err := r.cli.StreamIsAllowed(streamer); err != nil {
		return nil, err
	}
	if track != moq.SourceTrack {
		if _, ok := moq.ParseRenditionTrack(track); !ok {
			return nil, fmt.Errorf("invalid moq archive track %q", track)
		}
	}
	client, err := r.dialMoq(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	bc, err := client.RequestBroadcast(ctx, streamer)
	if err != nil {
		return nil, err
	}
	frame, err := fetchMoqFrame(ctx, bc, track, sequence)
	if err != nil {
		return nil, err
	}
	source := frame
	if track != moq.SourceTrack {
		if err := validateMoqRenditionTrack(ctx, track, frame.Payload); err != nil {
			return nil, err
		}
		source, err = fetchMoqFrame(ctx, bc, moq.SourceTrack, sequence)
		if err != nil {
			return nil, err
		}
		if err := r.mm.ValidateReplicatedRenditions(ctx, streamer, source.Payload, frame.Payload); err != nil {
			return nil, err
		}
	}
	meta, err := r.mm.ValidateArchivedMP4(ctx, bytes.NewReader(source.Payload), streamer)
	if err != nil {
		return nil, err
	}
	if err := r.storeMoqArchive(ctx, streamer, moq.SourceTrack, source.Payload, meta); err != nil {
		return nil, err
	}
	if track != moq.SourceTrack {
		if err := r.storeMoqArchive(ctx, streamer, track, frame.Payload, meta); err != nil {
			return nil, err
		}
	}
	return frame.Payload, nil
}

func (r *WebsocketReplicator) storeMoqArchive(ctx context.Context, streamer, name string, payload []byte, meta *media.SegmentMetadata) error {
	if meta == nil || !meta.Published || meta.Creator != streamer {
		return fmt.Errorf("archive requires verified public stream metadata")
	}
	timestampUs := uint64(meta.StartTime.Time().UnixMicro())
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := r.moqBroadcast(streamer)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	t, err := r.publishedTrack(b, streamer, name)
	if err != nil {
		return err
	}
	if name == moq.SourceTrack {
		expiresAt, err := moqArchiveExpiry(meta, time.Now(), r.cli.SegmentArchiveRetention)
		if err != nil {
			return err
		}
		_, err = t.archive.assignWithExpiry(timestampUs, payload, expiresAt)
		if err == nil {
			b.archiveEligible = true
		}
		return err
	}
	source, err := r.publishedTrack(b, streamer, moq.SourceTrack)
	if err != nil {
		return err
	}
	seq, ok := source.archive.sequenceAt(timestampUs)
	if !ok {
		return fmt.Errorf("no source group for archived rendition")
	}
	stored, ok, err := source.archive.get(seq)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("archived rendition source expired")
	}
	if err := t.archive.putWithExpiry(seq, timestampUs, payload, stored.expiresAt); err != nil {
		return err
	}
	return nil
}

func validateMoqRenditionTrack(ctx context.Context, name string, payload []byte) error {
	id, ok := moq.ParseRenditionTrack(name)
	if !ok {
		return fmt.Errorf("invalid rendition track %q", name)
	}
	pieces, err := moq.SplitRenditions(ctx, payload)
	if err != nil {
		return err
	}
	if len(pieces) != 1 || pieces[0].ID != id {
		return fmt.Errorf("signed MUXL track does not match announced track %q", name)
	}
	return nil
}
