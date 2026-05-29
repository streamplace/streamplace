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
	"stream.place/streamplace/pkg/streamplace"
)

func (mm *MediaManager) RTMPPush(ctx context.Context, user string, rendition string, targetView *streamplace.MultistreamDefs_TargetView) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithLogValues(ctx, "mediafunc", "RTMPPush")
	rec, ok := targetView.Record.Val.(*streamplace.MultistreamTarget)
	if !ok {
		return fmt.Errorf("failed to convert target view to multistream target")
	}
	targetURL := rec.Url

	pipelineSlice := []string{
		"appsrc name=muxlsrc ! qtdemux name=demux",
		"flvmux name=muxer ! rtmp2sink name=rtmp2sink",
		fmt.Sprintf("%s name=videoqueue ! h264parse ! muxer.video", constants.Queue2Big),
		// Segments carry AAC (we feed only the AAC track below), so pass it
		// straight to flvmux — no Opus→AAC transcode.
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

	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("failed to parse target URL: %w", err)
	}
	if u.Scheme == "rtmps" {
		localAddr, err := mm.RunTLSFForwarder(ctx, targetURL)
		if err != nil {
			return fmt.Errorf("failed to run TLS forwarder: %w", err)
		}
		local := fmt.Sprintf("rtmp://%s%s", localAddr, u.Path)
		log.Debug(ctx, "running TLS forwarder", "localAddr", local, "destination", targetURL)
		err = rtmp2sink.SetProperty("location", local)
		if err != nil {
			return fmt.Errorf("failed to set rtmp2sink location: %w", err)
		}
	} else if u.Scheme == "rtmp" {
		localAddr, err := mm.RunTCPForwarder(ctx, targetURL)
		if err != nil {
			return fmt.Errorf("failed to run TCP forwarder: %w", err)
		}
		local := fmt.Sprintf("rtmp://%s%s", localAddr, u.Path)
		log.Debug(ctx, "running TCP forwarder", "localAddr", local, "destination", targetURL)
		err = rtmp2sink.SetProperty("location", local)
		if err != nil {
			return fmt.Errorf("failed to set rtmp2sink location: %w", err)
		}
	} else {
		return fmt.Errorf("invalid target URL scheme: %s", u.Scheme)
	}

	go func() {
		pollFreq := time.Second * 1
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollFreq):
				prop, err := rtmp2sink.GetProperty("stats")
				if err != nil {
					log.Error(ctx, "error getting rtmp2sink peak-kbps", "error", err)
					continue
				}
				if prop == nil {
					log.Error(ctx, "failed to get rtmp2sink peak-kbps", "prop", prop)
					continue
				}
				propVal, ok := prop.(*gst.Structure)
				if !ok {
					log.Error(ctx, "failed to convert rtmp2sink peak-kbps", "prop", prop)
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
					err = mm.atsync.StatefulDB.CreateMultistreamEvent(targetView.Uri, fmt.Sprintf("wrote %d bytes", outBytesAckedVal), "active")
					if err != nil {
						log.Error(ctx, "failed to create multistream event", "error", err)
					}
					// increase pollFreq, once it's working we don't need to spam the database
					pollFreq = time.Second * 15
				}
				log.Debug(ctx, "rtmp2sink out-bytes-acked", "outBytesAckedVal", outBytesAckedVal)
			}

		}
	}()

	// Reassemble the streamer's source segments into one continuous fMP4
	// stream for a single qtdemux: synthesize the init (ftyp+moov) from the
	// first segment's embedded catalog, then blindly concatenate every
	// segment's canonical bytes after it. MUXL segments carry per-track
	// monotonic tfdt, so this is a valid fMP4 timeline with no remux.
	//
	// The init reflects the first segment's catalog and is never re-emitted.
	// muxl derives the catalog from the moov and does not parse the H.264
	// bitstream, so a mid-stream resolution/orientation change (carried in-band
	// as new SPS/PPS at a keyframe) is invisible to it: no second moov appears,
	// the parameter sets pass through verbatim to h264parse/flvmux, and the
	// init's declared dimensions simply stay at the initial config. Reflecting
	// such a change in container metadata would require parsing SPS/PPS.
	pr, pw := io.Pipe()
	muxlLoop := func() {
		segChan := mm.bus.SubscribeSegment(ctx, user, rendition)
		defer mm.bus.UnsubscribeSegment(ctx, user, rendition, segChan)
		first := true
		for {
			select {
			case <-ctx.Done():
				pw.CloseWithError(ctx.Err())
				return
			case seg := <-segChan.C:
				log.Debug(ctx, "segment received", "file", seg.Filepath)
				if len(seg.Muxl) == 0 {
					log.Warn(ctx, "source segment has no MUXL bytes, skipping", "file", seg.Filepath)
					continue
				}
				// RTMP wants AAC: select video + the AAC audio track from the
				// dual-codec segment and feed only those, so flvmux gets AAC
				// with no transcode.
				aacSeg, err := filterSegmentToCodec(ctx, seg.Muxl, false)
				if err != nil {
					log.Error(ctx, "failed to select AAC audio", "error", err)
					pw.CloseWithError(err)
					return
				}
				if first {
					var init bytes.Buffer
					if err := muxl.RunMuxlWrapInit(ctx, bytes.NewReader(aacSeg), &init); err != nil {
						pw.CloseWithError(fmt.Errorf("synthesize init segment: %w", err))
						return
					}
					log.Debug(ctx, "init segment synthesized", "size", init.Len())
					if _, err := pw.Write(init.Bytes()); err != nil {
						log.Error(ctx, "failed to write init segment", "error", err)
						pw.CloseWithError(err)
						return
					}
					first = false
				}
				log.Debug(ctx, "writing segment", "size", len(aacSeg))
				if _, err := pw.Write(aacSeg); err != nil {
					log.Error(ctx, "failed to write segment", "error", err)
					pw.CloseWithError(err)
					return
				}
			}
		}
	}
	go muxlLoop()

	muxlSrc, err := pipeline.GetElementByName("muxlsrc")
	if err != nil {
		return fmt.Errorf("failed to get appsrc element from pipeline: %w", err)
	}
	app.SrcFromElement(muxlSrc).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, pr),
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
		err = pipeline.SetState(gst.StateNull)
		if err != nil {
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
