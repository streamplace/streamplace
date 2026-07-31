package media

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
)

// RTMPPush is the in-process multistream egress: it assembles the streamer's
// source segments into a continuous fMP4 stream and runs the native RTMP push
// pipeline over it, reporting status straight to the DB. The isolated
// counterpart (RTMPPushIsolated) runs the same native pipeline in a worker
// subprocess so a gst fault in the egress chain can't take the node down.
func (mm *MediaManager) RTMPPush(ctx context.Context, user string, rendition string, targetView *placestream.MultistreamDefs_TargetView) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithLogValues(ctx, "mediafunc", "RTMPPush")
	rec, ok := targetView.Record.Val.(*placestream.MultistreamTarget)
	if !ok {
		return fmt.Errorf("failed to convert target view to multistream target")
	}

	// Source: subscribe to the streamer's segments and assemble one continuous
	// fMP4 stream for the push pipeline. Tied to ctx so it tears down when the
	// pipeline returns.
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(mm.writeRTMPSource(ctx, user, rendition, pw))
	}()

	// Status straight to the DB (in-process). The isolated worker reports the
	// same events back over a frame channel instead, where the supervisor writes
	// them — see runRTMPPushPipeline / RTMPPushIsolated.
	report := func(status, message string) {
		if err := mm.atsync.StatefulDB.CreateMultistreamEvent(targetView.Uri, message, status); err != nil {
			log.Error(ctx, "failed to create multistream event", "error", err)
		}
	}
	return mm.runRTMPPushPipeline(ctx, pr, rec.Url, report)
}

// writeRTMPSource subscribes to the streamer's source segments, selects the AAC
// audio + video tracks from each dual-codec segment, synthesizes a single fMP4
// init from the first segment, and writes one continuous fMP4 stream to w (init
// then every segment's canonical bytes concatenated). It returns when ctx is
// done or a select/encode/write fails; the caller owns closing w.
//
// MUXL segments carry per-track monotonic tfdt, so blind concatenation after a
// single synthesized init is a valid fMP4 timeline with no remux. The init
// reflects the first segment's catalog and is never re-emitted; muxl derives the
// catalog from the moov and does not parse the H.264 bitstream, so a mid-stream
// resolution/orientation change (carried in-band as new SPS/PPS at a keyframe)
// is invisible to it — the parameter sets pass through verbatim to
// h264parse/flvmux and the init's declared dimensions simply stay at the initial
// config. Reflecting such a change in container metadata would require parsing
// SPS/PPS.
func (mm *MediaManager) writeRTMPSource(ctx context.Context, user, rendition string, w io.Writer) error {
	segChan := mm.bus.SubscribeSegment(ctx, user, rendition)
	defer mm.bus.UnsubscribeSegment(ctx, user, rendition, segChan)
	first := true
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case seg := <-segChan.C:
			log.Debug(ctx, "segment received", "file", seg.Filepath)
			if len(seg.Muxl) == 0 {
				log.Warn(ctx, "source segment has no MUXL bytes, skipping", "file", seg.Filepath)
				continue
			}
			// RTMP wants AAC: select video + the AAC audio track from the
			// dual-codec segment and feed only those, so flvmux gets AAC with no
			// transcode.
			aacSeg, err := filterSegmentToCodec(ctx, seg.Muxl, false)
			if err != nil {
				return fmt.Errorf("select AAC audio: %w", err)
			}
			if first {
				var init bytes.Buffer
				if err := muxl.RunMuxlWrapInit(ctx, bytes.NewReader(aacSeg), &init); err != nil {
					return fmt.Errorf("synthesize init segment: %w", err)
				}
				log.Debug(ctx, "init segment synthesized", "size", init.Len())
				if _, err := w.Write(init.Bytes()); err != nil {
					return err
				}
				first = false
			}
			log.Debug(ctx, "writing segment", "size", len(aacSeg))
			if _, err := w.Write(aacSeg); err != nil {
				return err
			}
		}
	}
}

// runRTMPPushPipeline builds and runs the native RTMP egress pipeline:
// appsrc(source) → qtdemux → {h264parse, aacparse} → flvmux → rtmp2sink → a
// local TCP/TLS forwarder → the target URL. Status updates (currently "active"
// once the server acks bytes) go through report. This is the crash-prone native
// core shared by the in-process RTMPPush and the isolated rtmp-push worker; only
// `source` and `report` differ between them, so the two paths run an identical
// gst pipeline and can't drift.
func (mm *MediaManager) runRTMPPushPipeline(ctx context.Context, source io.Reader, targetURL string, report func(status, message string)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		"appsrc name=muxlsrc ! qtdemux name=demux",
		"flvmux name=muxer ! rtmp2sink name=rtmp2sink",
		fmt.Sprintf("%s name=videoqueue ! h264parse ! muxer.video", constants.Queue2Big),
		// Segments carry AAC (we feed only the AAC track), so pass it straight to
		// flvmux — no Opus→AAC transcode.
		fmt.Sprintf("%s name=audioqueue ! aacparse ! muxer.audio", constants.Queue2Big),
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err) //nolint:all
	}

	rtmp2sink, err := pipeline.GetElementByName("rtmp2sink")
	if err != nil {
		return fmt.Errorf("failed to get rtmp2sink element from pipeline: %w", err)
	}

	// rtmp2sink can't speak rtmps and doesn't do TLS, so we always point it at a
	// localhost forwarder that relays (optionally over TLS) to the real target.
	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("failed to parse target URL: %w", err)
	}
	var localAddr string
	switch u.Scheme {
	case "rtmps":
		localAddr, err = mm.RunTLSFForwarder(ctx, targetURL)
	case "rtmp":
		localAddr, err = mm.RunTCPForwarder(ctx, targetURL)
	default:
		return fmt.Errorf("invalid target URL scheme: %s", u.Scheme)
	}
	if err != nil {
		return fmt.Errorf("failed to run forwarder: %w", err)
	}
	local := fmt.Sprintf("rtmp://%s%s", localAddr, u.Path)
	log.Debug(ctx, "running forwarder", "localAddr", local)
	if err := rtmp2sink.SetProperty("location", local); err != nil {
		return fmt.Errorf("failed to set rtmp2sink location: %w", err)
	}

	// Poll the sink and report "active" once the destination acks bytes; back off
	// once it's flowing so we don't spam the status channel.
	go func() {
		pollFreq := time.Second * 1
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollFreq):
				prop, err := rtmp2sink.GetProperty("stats")
				if err != nil {
					log.Error(ctx, "error getting rtmp2sink stats", "error", err)
					continue
				}
				if prop == nil {
					log.Error(ctx, "failed to get rtmp2sink stats", "prop", prop)
					continue
				}
				propVal, ok := prop.(*gst.Structure)
				if !ok {
					log.Error(ctx, "failed to convert rtmp2sink stats", "prop", prop)
					continue
				}
				outBytesAcked, err := propVal.GetValue("out-bytes-acked")
				if err != nil {
					log.Error(ctx, "failed to get rtmp2sink out-bytes-acked", "error", err)
					continue
				}
				outBytesAckedVal, ok := outBytesAcked.(uint64)
				if !ok {
					log.Error(ctx, "failed to convert rtmp2sink out-bytes-acked", "prop", prop)
					continue
				}
				if outBytesAckedVal > 0 {
					report("active", fmt.Sprintf("wrote %d bytes", outBytesAckedVal))
					// once it's working we don't need to spam the status channel
					pollFreq = time.Second * 15
				}
				log.Debug(ctx, "rtmp2sink out-bytes-acked", "outBytesAckedVal", outBytesAckedVal)
			}
		}
	}()

	muxlSrc, err := pipeline.GetElementByName("muxlsrc")
	if err != nil {
		return fmt.Errorf("failed to get appsrc element from pipeline: %w", err)
	}
	app.SrcFromElement(muxlSrc).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, source),
	})

	videoQueue, err := pipeline.GetElementByName("videoqueue")
	if err != nil {
		return fmt.Errorf("failed to get video parse element from pipeline: %w", err)
	}
	audioQueue, err := pipeline.GetElementByName("audioqueue")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}

	// qtdemux exposes its track pads only after parsing the moov, so link them
	// on pad-added: video → h264parse, audio → aacparse.
	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return fmt.Errorf("failed to get demux element from pipeline: %w", err)
	}
	if _, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
		name := pad.GetName()
		var sink *gst.Pad
		switch {
		case strings.HasPrefix(name, "video_"):
			sink = videoQueue.GetStaticPad("sink")
		case strings.HasPrefix(name, "audio_"):
			sink = audioQueue.GetStaticPad("sink")
		default:
			log.Debug(ctx, "ignoring demux pad", "name", name)
			return
		}
		if linked := pad.Link(sink); linked != gst.PadLinkOK {
			log.Error(ctx, "failed to link demux pad", "name", name, "result", linked)
		}
	}); err != nil {
		return fmt.Errorf("failed to connect demux pad-added: %w", err)
	}

	errCh := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		log.Log(ctx, "RTMP push pipeline error", "error", err)
		errCh <- err
	}()

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return fmt.Errorf("failed to set pipeline state to playing: %w", err)
	}

	defer func() {
		log.Log(ctx, "shutting down RTMP push pipeline")
		if err := pipeline.SetState(gst.StateNull); err != nil {
			log.Error(ctx, "failed to set pipeline state to null", "error", err)
		}
	}()

	return <-errCh
}

func (mm *MediaManager) RunTLSFForwarder(ctx context.Context, dest string) (string, error) {
	destURL, err := url.Parse(dest)
	if err != nil {
		return "", fmt.Errorf("failed to parse destination URL: %w", err)
	}
	return mm.runForwarder(ctx, dest, func(destHost string) (net.Conn, error) {
		return tls.Dial("tcp", destHost, &tls.Config{
			ServerName: destURL.Hostname(),
		})
	})
}

func (mm *MediaManager) RunTCPForwarder(ctx context.Context, dest string) (string, error) {
	return mm.runForwarder(ctx, dest, func(destHost string) (net.Conn, error) {
		return net.Dial("tcp", destHost)
	})
}

func (mm *MediaManager) runForwarder(ctx context.Context, dest string, dial func(destHost string) (net.Conn, error)) (string, error) {
	ctx = log.WithLogValues(ctx, "mediafunc", "runForwarder")
	// Parse the destination URL to extract host and port
	destURL, err := url.Parse(dest)
	if err != nil {
		return "", fmt.Errorf("failed to parse destination URL: %w", err)
	}

	// Default to port 1935 if not specified
	destHost := destURL.Host
	if !strings.Contains(destHost, ":") {
		destHost = destHost + ":1935"
	}

	// Listen on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to listen on random port: %w", err)
	}

	log.Debug(ctx, "RTMP forwarder listening", "localAddr", listener.Addr().String(), "destination", dest)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	go func() {
		defer listener.Close()
		if ctx.Err() != nil {
			return
		}
		// Accept incoming RTMP connection
		clientConn, err := listener.Accept()
		if err != nil {
			log.Error(ctx, "failed to accept connection", "error", err)
			return
		}

		closed := false
		go func() {
			<-ctx.Done()
			if !closed {
				closed = true
				clientConn.Close()
			}
		}()

		defer func() {
			if !closed {
				closed = true
				clientConn.Close()
			}
		}()

		// Establish connection to destination
		serverConn, err := dial(destHost)
		if err != nil {
			log.Error(ctx, "failed to establish connection to destination", "error", err)
			return
		}
		defer serverConn.Close()

		// Proxy data bidirectionally
		done := make(chan error, 2)

		// Copy from client to server
		go func() {
			_, err := io.Copy(serverConn, clientConn)
			done <- err
		}()

		// Copy from server to client
		go func() {
			_, err := io.Copy(clientConn, serverConn)
			done <- err
		}()

		// Wait for either direction to complete or error
		err = <-done
		if err != nil {
			log.Error(ctx, "proxy connection error", "error", err)
		}
	}()

	return listener.Addr().String(), nil
}
