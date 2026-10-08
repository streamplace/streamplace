package media

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

// This experimental policy permits small scheduling jitter, not late-packet
// catchup. A delayed write changes Pion's sender-report wall-clock mapping.
const earlyAACMaximumLateness = 50 * time.Millisecond

func earlyRTPTicks(duration time.Duration, rate uint32) uint32 {
	return uint32(uint64(duration/time.Second)*uint64(rate) + uint64(duration%time.Second)*uint64(rate)/uint64(time.Second))
}

func writeEarlyAACPackets(track *webrtc.TrackLocalStaticRTP, packetizer rtp.Packetizer, sample bus.PacketizedSample, origin time.Duration, rate uint32) error {
	ts := sample.Timestamp - origin
	if !sample.HasTimestamp || origin < 0 || sample.Timestamp < origin || ts < 0 || sample.Duration <= 0 || rate == 0 {
		return fmt.Errorf("early AAC invalid sample clock")
	}
	ticks := earlyRTPTicks(sample.Duration, rate)
	for _, packet := range packetizer.Packetize(sample.Data, ticks) {
		packet.Timestamp = earlyRTPTicks(ts, rate)
		if err := track.WriteRTP(packet); err != nil {
			return err
		}
	}
	return nil
}

func writeScheduledEarlyAAC(ctx context.Context, track *webrtc.TrackLocalStaticRTP, packetizer rtp.Packetizer, sample bus.PacketizedSample, origin time.Duration, rate uint32, wallOrigin time.Time) error {
	if origin < 0 || sample.Timestamp < origin {
		return fmt.Errorf("early AAC invalid scheduled clock")
	}
	due := wallOrigin.Add(sample.Timestamp - origin)
	if late := time.Since(due); late > earlyAACMaximumLateness {
		return fmt.Errorf("early AAC packet materially late: %s", late)
	}
	timer := time.NewTimer(time.Until(due))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return ctx.Err()
	}
	if late := time.Since(due); late > earlyAACMaximumLateness {
		return fmt.Errorf("early AAC packet materially late: %s", late)
	}
	return writeEarlyAACPackets(track, packetizer, sample, origin, rate)
}

func (mm *MediaManager) webRTCEarlyAAC(ctx context.Context, user string, offer *webrtc.SessionDescription, s *earlyAACSession, p *earlyAACPeer) (*webrtc.SessionDescription, error) {
	log.Log(context.Background(), "experimental early AAC selected", "user", user, "epoch", s.epoch)
	pc, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		s.leave(p)
		return nil, err
	}
	fail := func(err error) (*webrtc.SessionDescription, error) { s.leave(p); pc.Close(); return nil, err }
	video, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}, "video", "pion")
	if err != nil {
		return fail(err)
	}
	audio, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "pion")
	if err != nil {
		return fail(err)
	}
	for _, track := range []*webrtc.TrackLocalStaticRTP{video, audio} {
		sender, e := pc.AddTrack(track)
		if e != nil {
			return fail(e)
		}
		go func() {
			b := make([]byte, 1500)
			for {
				if _, _, e := sender.Read(b); e != nil {
					return
				}
			}
		}()
	}
	connected := make(chan struct{}, 1)
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateConnected:
			select {
			case connected <- struct{}{}:
			default:
			}
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected:
			p.cancel()
		}
	})
	if err = pc.SetRemoteDescription(*offer); err != nil {
		return fail(err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return fail(err)
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		return fail(err)
	}
	go func() {
		defer s.leave(p)
		defer pc.Close()
		select {
		case <-connected:
		case <-ctx.Done():
			return
		case <-p.ctx.Done():
			return
		}
		mm.IncrementViewerCount(user, "webrtc")
		defer mm.DecrementViewerCount(user, "webrtc")
		runCtx, cancel := context.WithCancel(p.ctx)
		defer cancel()
		var failureOnce sync.Once
		failPlayback := func(err error) {
			failureOnce.Do(func() {
				log.Log(context.Background(), "experimental early AAC playback failed", "user", user, "epoch", s.epoch, "reason", err)
			})
			cancel()
		}
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-runCtx.Done():
			}
		}()
		queues := [2]chan earlyAACSample{make(chan earlyAACSample, earlySampleLimit), make(chan earlyAACSample, earlySampleLimit)}
		var queuedBytes [2]atomic.Int64
		// The atomic startup cache guarantees a video origin. Worker clocks share
		// it even when one track temporarily has no newly available packets.
		var first earlyAACSample
		select {
		case first = <-p.queue:
			s.take(p, first)
		case <-runCtx.Done():
			return
		}
		mediaOrigin := first.sample.Timestamp
		wallOrigin := time.Now().Add(20 * time.Millisecond)
		var writers sync.WaitGroup
		writersDone := make(chan struct{})
		defer func() {
			cancel()
			pc.Close()
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-writersDone:
			case <-timer.C:
			}
		}()
		for i, track := range []*webrtc.TrackLocalStaticRTP{audio, video} {
			rate := uint32(48000)
			var payloader rtp.Payloader = &codecs.OpusPayloader{}
			if i == 1 {
				rate = 90000
				payloader = &codecs.H264Payloader{}
			}
			writers.Add(1)
			go func(index int, track *webrtc.TrackLocalStaticRTP, rate uint32, payloader rtp.Payloader) {
				defer writers.Done()
				packetizer := rtp.NewPacketizerWithOptions(1200, payloader, rtp.NewRandomSequencer(), rate, rtp.WithTimestamp(0))
				var previous time.Duration
				started := false
				for {
					select {
					case <-runCtx.Done():
						return
					case item, ok := <-queues[index]:
						if !ok {
							return
						}
						queuedBytes[index].Add(-int64(len(item.sample.Data)))
						ts := item.sample.Timestamp - mediaOrigin
						if ts < 0 || item.sample.Duration <= 0 || item.sample.Duration > time.Duration(math.MaxInt64)-ts || (started && ts < previous && previous-ts > time.Millisecond) {
							failPlayback(fmt.Errorf("early AAC nonmonotonic packet clock"))
							return
						}
						started = true
						if e := writeScheduledEarlyAAC(runCtx, track, packetizer, item.sample, mediaOrigin, rate, wallOrigin); e != nil {
							failPlayback(e)
							return
						}
						previous = ts + item.sample.Duration
					}
				}
			}(i, track, rate, payloader)
		}
		go func() { writers.Wait(); close(writersDone) }()
		send := func(item earlyAACSample) bool {
			index := 0
			if item.video {
				index = 1
			}
			if queuedBytes[index].Add(int64(len(item.sample.Data))) > earlyByteLimit {
				cancel()
				return false
			}
			select {
			case queues[index] <- item:
				return true
			default:
				cancel()
				return false
			}
		}
		if !send(first) {
			return
		}
		for {
			select {
			case <-runCtx.Done():
				return
			case item, ok := <-p.queue:
				if !ok {
					close(queues[0])
					close(queues[1])
					timer := time.NewTimer(8 * time.Second)
					defer timer.Stop()
					select {
					case <-writersDone:
					case <-runCtx.Done():
					case <-timer.C:
						failPlayback(fmt.Errorf("early AAC peer drain timeout"))
					}
					return
				}
				s.take(p, item)
				if !send(item) {
					return
				}
			}
		}
	}()
	select {
	case <-gathered:
		return pc.LocalDescription(), nil
	case <-ctx.Done():
		return fail(ctx.Err())
	case <-p.ctx.Done():
		return fail(fmt.Errorf("early AAC session ended: %w", p.ctx.Err()))
	}
}
