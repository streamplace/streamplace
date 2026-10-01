package media

import (
	"context"
	"fmt"
	"path/filepath"
	"stream.place/streamplace/pkg/stt"
)

// StartCaptionEngineProxy shares the node scheduler with isolated media workers.
func (mm *MediaManager) StartCaptionEngineProxy(ctx context.Context) (func(), error) {
	dir, err := mm.ingestWorkerSocketDir()
	if err != nil {
		return nil, err
	}
	// Detached-worker discovery reserves the .sock suffix for frame sockets.
	path := filepath.Join(dir, "speech-engine.stt")
	stop, err := stt.ServeEngine(ctx, path, mm.STT)
	if err != nil {
		return nil, fmt.Errorf("serve node speech engine: %w", err)
	}
	mm.CaptionEngineSocket = path
	return stop, nil
}
