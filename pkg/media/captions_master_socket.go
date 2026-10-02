package media

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"stream.place/streamplace/pkg/captions"
)

// Detached ingest owns its media clock and master. A separate private control
// socket avoids replacing the main segment connection when CART tools push.
type captionControl struct {
	Track captions.Track
	Cues  []captions.Cue
}
type captionControlReply struct {
	Policy captions.Policy
	Error  string
}
type remoteCaptionMaster struct{ path string }

func (m *captionMaster) servePush(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	socket := path + ".captions"
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0600); err != nil {
		ln.Close()
		os.Remove(socket)
		return nil, err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				var req captionControl
				if json.NewDecoder(conn).Decode(&req) != nil {
					return
				}
				reply := captionControlReply{Policy: m.policy()}
				if len(req.Cues) > 0 {
					if err := m.push(req.Track, req.Cues); err != nil {
						reply.Error = err.Error()
					}
				}
				_ = json.NewEncoder(conn).Encode(reply)
			}()
		}
	}()
	return func() { ln.Close(); os.Remove(socket) }, nil
}
func (r *remoteCaptionMaster) call(req captionControl) (captionControlReply, error) {
	conn, err := net.DialTimeout("unix", r.path+".captions", 2*time.Second)
	if err != nil {
		return captionControlReply{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return captionControlReply{}, err
	}
	var reply captionControlReply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		return reply, err
	}
	if reply.Error != "" {
		return reply, fmt.Errorf("caption worker: %s", reply.Error)
	}
	return reply, nil
}
func (r *remoteCaptionMaster) captionPolicy() (captions.Policy, error) {
	reply, err := r.call(captionControl{})
	return reply.Policy, err
}
func (r *remoteCaptionMaster) push(track captions.Track, cues []captions.Cue) error {
	_, err := r.call(captionControl{Track: track, Cues: cues})
	return err
}

// registerWorkerCaptionMaster routes pushes to a worker's caption socket. When
// main records segments, the returned ctx also collects the worker's archival
// captions (Captions frames) for them; a ctx that already does is reused.
func (mm *MediaManager) registerWorkerCaptionMaster(ctx context.Context, path, streamer string) (context.Context, func()) {
	unregister := mm.registerCaptionMaster(streamer, &remoteCaptionMaster{path: path})
	if captionArchiveFrom(ctx) != nil || mm.cli == nil || !mm.cli.S3Configured() {
		return ctx, unregister
	}
	archive := newCaptionArchive(mm.cli.CaptionsMasterDelay)
	return withCaptionArchive(ctx, archive), func() { unregister(); archive.finish() }
}
