package aigateway

import (
	"context"
	"io"
	"os/exec"
	"sync"

	"stream.place/streamplace/pkg/log"
)

// RTMPPublisher publishes media to an RTMP endpoint using ffmpeg.
// It transcodes incoming MKV data to FLV format suitable for RTMP ingestion.
type RTMPPublisher struct {
	ctx        context.Context
	cancel     context.CancelFunc
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	wg         sync.WaitGroup
	ffmpegBin  string
	rtmpURL    string
	started    bool
	startedMu  sync.Mutex
	err        error
}

// NewRTMPPublisher creates a new RTMP publisher that will stream to the given URL.
// If ffmpegBin is empty, it defaults to "ffmpeg".
func NewRTMPPublisher(ctx context.Context, ffmpegBin, rtmpURL string) *RTMPPublisher {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	ctx, cancel := context.WithCancel(ctx)
	return &RTMPPublisher{
		ctx:       ctx,
		cancel:    cancel,
		ffmpegBin: ffmpegBin,
		rtmpURL:   rtmpURL,
	}
}

// Start begins the ffmpeg process and returns a WriteCloser for sending MKV data.
// Returns nil, nil if already started. The returned writer accepts MKV formatted data.
func (p *RTMPPublisher) Start() (io.WriteCloser, error) {
	p.startedMu.Lock()
	defer p.startedMu.Unlock()

	if p.started {
		return p.stdin, nil
	}

	p.cmd = exec.CommandContext(p.ctx, p.ffmpegBin,
		"-hide_banner",
		"-loglevel", "warning",
		"-f", "matroska",
		"-i", "pipe:0",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-c:a", "aac",
		"-ar", "44100",
		"-f", "flv",
		p.rtmpURL,
	)

	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	if err := p.cmd.Start(); err != nil {
		return nil, err
	}

	p.started = true
	log.Log(p.ctx, "ffmpeg RTMP publisher started", "pid", p.cmd.Process.Pid, "rtmpURL", p.rtmpURL)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		err := p.cmd.Wait()
		if err != nil && p.ctx.Err() == nil {
			log.Error(p.ctx, "ffmpeg RTMP publisher exited with error", "error", err)
			p.err = err
		} else {
			log.Log(p.ctx, "ffmpeg RTMP publisher exited")
		}
	}()

	return p.stdin, nil
}

// Stop terminates the ffmpeg process and waits for it to exit.
func (p *RTMPPublisher) Stop() {
	p.cancel()

	p.startedMu.Lock()
	started := p.started
	stdin := p.stdin
	p.startedMu.Unlock()

	if !started {
		return
	}

	if stdin != nil {
		_ = stdin.Close()
	}

	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}

	p.wg.Wait()
}

// Error returns any error that occurred during ffmpeg execution.
// This should be checked after Stop() returns.
func (p *RTMPPublisher) Error() error {
	p.startedMu.Lock()
	defer p.startedMu.Unlock()
	return p.err
}
