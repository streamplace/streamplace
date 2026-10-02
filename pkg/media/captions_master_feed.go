package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/fmp4"
	"stream.place/streamplace/pkg/log"
)

// tee stages a normal hold independently of the signer. Each bounded queue
// backpressures sustained overload rather than retaining unlimited media.
func (m *captionMaster) tee(input io.Reader) (io.Reader, func()) {
	media, audio := newIngestByteBuffer(m.ctx), newIngestByteBuffer(m.ctx)
	done := make(chan struct{})
	go func() {
		select {
		case <-m.ctx.Done():
			_ = media.CloseWithError(m.ctx.Err())
			_ = audio.CloseWithError(m.ctx.Err())
			if closer, ok := input.(io.ReadCloser); ok {
				closer.Close()
			}
		case <-done:
		}
	}()
	go func() {
		defer close(done)
		if err := m.readMedia(audio); err != nil {
			log.Warn(m.ctx, "caption ingest tap stopped", "error", err)
		}
		// Continue draining if captions fail; media must still be signed.
		_, _ = io.Copy(io.Discard, audio)
	}()
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, err := input.Read(buf)
			if n > 0 {
				at := captionReadTime(input)
				if _, writeErr := media.writeAt(buf[:n], at); writeErr != nil {
					return
				}
				_, _ = audio.writeAt(buf[:n], at)
			}
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				_ = media.CloseWithError(err)
				_ = audio.CloseWithError(err)
				return
			}
		}
	}()
	return media, func() {
		m.stop()
		_ = media.CloseWithError(context.Canceled)
		_ = audio.CloseWithError(context.Canceled)
		if closer, ok := input.(io.ReadCloser); ok {
			closer.Close()
		}
		<-done
	}
}

func readCaptionBox(r io.Reader) ([]byte, string, error) {
	var h [8]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, "", err
	}
	size := uint64(binary.BigEndian.Uint32(h[:4]))
	header := 8
	var extra [8]byte
	if size == 1 {
		if _, err := io.ReadFull(r, extra[:]); err != nil {
			return nil, "", err
		}
		size = binary.BigEndian.Uint64(extra[:])
		header = 16
	}
	if size < uint64(header) || size > 256*1024*1024 {
		return nil, "", fmt.Errorf("invalid caption MP4 box size %d", size)
	}
	data := make([]byte, int(size))
	copy(data, h[:])
	if header == 16 {
		copy(data[8:], extra[:])
	}
	if _, err := io.ReadFull(r, data[header:]); err != nil {
		return nil, "", err
	}
	return data, string(h[4:]), nil
}

// recognizerLease is a recognizer readMedia started, once its engine lease
// resolves.
type recognizerLease struct {
	recognizer *captions.Recognizer
	err        error
}

func (m *captionMaster) readMedia(input io.Reader) error {
	var init, moof []byte
	var tracks []fmp4.TrackInfo
	var referenceVideo, referenceAudio uint32
	var decoder *captionAudioDecoder
	var recognizer *captions.Recognizer
	// The signer waits on this parse (segmentTime), so it waits for a
	// recognizer's lease no longer than for the tap: a ready engine answers at
	// once, while one still measuring its models (node startup) leaves the
	// audio before it answers unrecognized.
	var leasing chan recognizerLease
	unavailable := false
	var last time.Time
	tap := captions.NewIngestTap(m.streamer, m.hub, captions.OriginCanonical, m.streamer, "und")
	defer m.finishMedia()
	stopRecognition := func() {
		if leasing != nil {
			go func(pending chan recognizerLease) {
				if lease := <-pending; lease.recognizer != nil {
					lease.recognizer.Close()
				}
			}(leasing)
			leasing = nil
		}
		if decoder != nil {
			decoder.close()
			decoder = nil
		}
		if recognizer != nil {
			recognizer.Close()
			recognizer = nil
		}
	}
	defer func() {
		stopRecognition()
		tap.Close(last)
	}()
	for {
		data, kind, err := readCaptionBox(input)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch kind {
		case "ftyp":
			init = data
		case "moov":
			init = append(init, data...)
			tracks, err = fmp4.Tracks(init)
			if err != nil {
				return err
			}
			for _, track := range tracks {
				if track.Handler == "vide" && (referenceVideo == 0 || track.ID < referenceVideo) {
					referenceVideo = track.ID
				}
				if track.Handler == "soun" && (referenceAudio == 0 || track.ID < referenceAudio) {
					referenceAudio = track.ID
				}
			}
		case "moof":
			moof = data
		case "mdat":
			if len(moof) == 0 {
				continue
			}
			fragment := append(moof, data...)
			moof = nil
			frags, err := fmp4.Fragments(fragment)
			if err != nil {
				return err
			}
			for _, frag := range frags {
				for _, track := range tracks {
					if track.ID != frag.TrackID || track.Timescale == 0 {
						continue
					}
					mediaTime := captionTicksDuration(frag.BaseDecodeTime, track.Timescale)
					m.clockAt(time.UnixMilli(0).Add(mediaTime), captionReadTime(input))
					p, err := m.waitPolicy()
					if err != nil {
						return err
					}
					if referenceVideo == 0 && track.ID == referenceAudio && len(frag.Samples) > 0 {
						// Match muxl's audio reference when there is no video.
						// Reanchor at fragment arrival, but mark all parsed samples
						// ready without inventing video keyframe closures.
						m.closeGopAt(uint64(mediaTime/time.Millisecond), captionReadTime(input))
						until := uint64(captionTicksDuration(frag.Samples[len(frag.Samples)-1].DTS, track.Timescale) / time.Millisecond)
						m.mu.Lock()
						if until > m.parsedUntil {
							m.parsedUntil = until
							m.signal()
						}
						m.mu.Unlock()
					}
					if track.Handler == "vide" {
						language := "und"
						if len(p.Languages) > 0 {
							language = p.Languages[0]
						}
						tap.SetTrack(captions.OriginCanonical, m.streamer, language)
						tap.Publish(p.Canonical != captions.CanonicalOff)
						for _, sample := range frag.Samples {
							when := time.UnixMilli(0).Add(captionTicksDuration(sample.PTS, track.Timescale))
							last = when
							if sample.Sync && track.ID == referenceVideo {
								m.closeGopAt(uint64(captionTicksDuration(sample.DTS, track.Timescale)/time.Millisecond), captionReadTime(input))
							}
							tap.Sample(sample.Data, when)
						}
						if tap.Seen() {
							m.mu.Lock()
							m.ingestSeen = true
							m.signal()
							m.mu.Unlock()
						}
					}
					m.mu.Lock()
					decision := captions.Decide(captions.Situation{Policy: m.current, Origin: true, IngestCaptions: m.ingestSeen})
					m.mu.Unlock()
					canonicalRecognition := decision.Recognize() && decision.Origin == captions.OriginCanonical
					if !canonicalRecognition {
						stopRecognition()
					}
					if track.Handler == "soun" && canonicalRecognition && m.engine != nil && !unavailable {
						if decoder == nil {
							var lease recognizerLease
							if leasing == nil {
								leasing = make(chan recognizerLease, 1)
								go func(out chan<- recognizerLease, languages []string) {
									r, err := captions.NewRecognizer(context.WithoutCancel(m.ctx), captions.RecognizerOptions{Streamer: m.streamer, Origin: captions.OriginCanonical, Author: m.streamer, Languages: languages, Hub: m.hub, Engine: m.engine, OnCoverage: m.coverage})
									out <- recognizerLease{r, err}
								}(leasing, p.Languages)
								select {
								case lease = <-leasing:
								case <-time.After(liveTapWait):
									continue
								}
							} else {
								select {
								case lease = <-leasing:
								default:
									continue
								}
							}
							leasing = nil
							if lease.err != nil {
								log.Warn(m.ctx, "canonical recognizer unavailable", "error", lease.err)
								unavailable = true
								m.mu.Lock()
								m.recognitionUnavailable = true
								m.signal()
								m.mu.Unlock()
								continue
							}
							recognizer = lease.recognizer
							codec := "aac"
							if bytes.Contains(init, []byte("Opus")) {
								codec = "opus"
							}
							decoder, err = newCaptionAudioDecoder(context.WithoutCancel(m.ctx), codec, func(at time.Time, pcm []float32) {
								m.mu.Lock()
								decision := captions.Decide(captions.Situation{Policy: m.current, Origin: true, IngestCaptions: m.ingestSeen})
								m.mu.Unlock()
								if decision.Recognize() && decision.Origin == captions.OriginCanonical {
									recognizer.Push(at, pcm)
								}
							})
							if err != nil {
								return err
							}
						}
						if err := decoder.feedMedia(init, fragment, frag.BaseDecodeTime, mediaTime); err != nil {
							return err
						}
					}
				}
			}
		}
	}
}

// feedMedia is the origin-only synthetic clock mode. Distribution continues to
// call feed with each validated segment's wall-clock anchor unchanged.
func (d *captionAudioDecoder) feedMedia(init, seg []byte, tfdt uint64, media time.Duration) error {
	return d.feed(init, seg, tfdt, media, time.UnixMilli(0).Add(media))
}

func captionTicksDuration(ticks uint64, scale uint32) time.Duration {
	return time.Duration(ticks/uint64(scale))*time.Second + time.Duration(ticks%uint64(scale))*time.Second/time.Duration(scale)
}
