package media

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
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
		"flvmux name=muxer ! rtmp2sink name=rtmp2sink",
		"h264parse name=videoparse ! muxer.video",
		"opusparse name=audioparse ! opusdec ! fdkaacenc ! muxer.audio",
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

	segBuffer := make(chan *bus.Seg, 1024)
	go func() {
		segChan := mm.bus.SubscribeSegment(ctx, user, rendition)
		defer mm.bus.UnsubscribeSegment(ctx, user, rendition, segChan)
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case file := <-segChan.C:
				log.Debug(ctx, "got segment", "file", file.Filepath)
				segBuffer <- file
			}
		}
	}()

	segCh := make(chan *bus.Seg)
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case seg := <-segBuffer:
				select {
				case <-ctx.Done():
					return
				case segCh <- seg:
				}
			}
		}
	}()

	concatBin, err := ConcatBin(ctx, segCh, true)
	if err != nil {
		return fmt.Errorf("failed to create concat bin: %w", err)
	}

	err = pipeline.Add(concatBin.Element)
	if err != nil {
		return fmt.Errorf("failed to add concat bin to pipeline: %w", err)
	}

	videoPad := concatBin.GetStaticPad("video_0")
	if videoPad == nil {
		return fmt.Errorf("video pad not found")
	}

	audioPad := concatBin.GetStaticPad("audio_0")
	if audioPad == nil {
		return fmt.Errorf("audio pad not found")
	}

	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	videoParsePad := videoParse.GetStaticPad("sink")
	if videoParsePad == nil {
		return fmt.Errorf("video parse pad not found")
	}
	linked := videoPad.Link(videoParsePad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link video pad to video parse pad: %v", linked)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	audioParsePad := audioParse.GetStaticPad("sink")
	if audioParsePad == nil {
		return fmt.Errorf("audio parse pad not found")
	}
	linked = audioPad.Link(audioParsePad)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link audio pad to audio parse pad: %v", linked)
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
