package media

// func (mm *MediaManager) MP4Playback(ctx context.Context, user string, rendition string, w io.Writer) error {
// 	uu, err := uuid.NewV7()
// 	if err != nil {
// 		return err
// 	}
// 	ctx = log.WithLogValues(ctx, "playbackID", uu.String())
// 	ctx, cancel := context.WithCancel(ctx)

// 	ctx = log.WithLogValues(ctx, "mediafunc", "MP4Playback")

// 	pipelineSlice := []string{
// 		"mp4mux name=muxer fragment-mode=first-moov-then-finalise fragment-duration=1000 streamable=true ! appsink name=mp4sink",
// 		"h264parse name=videoparse ! muxer.",
// 		"opusparse name=audioparse ! muxer.",
// 	}

// 	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
// 	if err != nil {
// 		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
// 	}

// 	go func() {
// 		HandleBusMessages(ctx, pipeline)
// 		cancel()
// 	}()

// 	outputQueue, done, err := ConcatStream(ctx, pipeline, user, rendition, mm)
// 	if err != nil {
// 		return fmt.Errorf("failed to get output queue: %w", err)
// 	}
// 	go func() {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-done:
// 			cancel()
// 		}
// 	}()

// 	videoParse, err := pipeline.GetElementByName("videoparse")
// 	if err != nil {
// 		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
// 	}
// 	err = outputQueue.Link(videoParse)
// 	if err != nil {
// 		return fmt.Errorf("failed to link output queue to video parse: %w", err)
// 	}

// 	audioParse, err := pipeline.GetElementByName("audioparse")
// 	if err != nil {
// 		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
// 	}
// 	err = outputQueue.Link(audioParse)
// 	if err != nil {
// 		return fmt.Errorf("failed to link output queue to audio parse: %w", err)
// 	}

// 	go func() {
// 		ticker := time.NewTicker(time.Second * 1)
// 		for {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			case <-ticker.C:
// 				state := pipeline.GetCurrentState()
// 				log.Debug(ctx, "pipeline state", "state", state)
// 			}
// 		}
// 	}()

// 	mp4sinkele, err := pipeline.GetElementByName("mp4sink")
// 	if err != nil {
// 		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
// 	}
// 	mp4sink := app.SinkFromElement(mp4sinkele)
// 	mp4sink.SetCallbacks(&app.SinkCallbacks{
// 		NewSampleFunc: WriterNewSample(ctx, w),
// 		EOSFunc: func(sink *app.Sink) {
// 			log.Warn(ctx, "mp4sink EOSFunc")
// 			cancel()
// 		},
// 	})

// 	pipeline.SetState(gst.StatePlaying)

// 	<-ctx.Done()

// 	pipeline.BlockSetState(gst.StateNull)

// 	return nil
// }

// func (mm *MediaManager) MKVPlayback(ctx context.Context, user string, rendition string, w io.Writer) error {
// 	uu, err := uuid.NewV7()
// 	if err != nil {
// 		return err
// 	}
// 	ctx = log.WithLogValues(ctx, "playbackID", uu.String())
// 	ctx, cancel := context.WithCancel(ctx)

// 	ctx = log.WithLogValues(ctx, "mediafunc", "MKVPlayback")

// 	pipelineSlice := []string{
// 		"matroskamux name=muxer streamable=true ! appsink name=mkvsink",
// 		"h264parse name=videoparse ! muxer.",
// 		"opusparse name=audioparse ! muxer.",
// 	}

// 	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
// 	if err != nil {
// 		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
// 	}

// 	go func() {
// 		HandleBusMessages(ctx, pipeline)
// 		cancel()
// 	}()

// 	outputQueue, done, err := ConcatStream(ctx, pipeline, user, rendition, mm)
// 	if err != nil {
// 		return fmt.Errorf("failed to get output queue: %w", err)
// 	}
// 	go func() {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-done:
// 			cancel()
// 		}
// 	}()

// 	videoParse, err := pipeline.GetElementByName("videoparse")
// 	if err != nil {
// 		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
// 	}
// 	err = outputQueue.Link(videoParse)
// 	if err != nil {
// 		return fmt.Errorf("failed to link output queue to video parse: %w", err)
// 	}

// 	audioParse, err := pipeline.GetElementByName("audioparse")
// 	if err != nil {
// 		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
// 	}
// 	err = outputQueue.Link(audioParse)
// 	if err != nil {
// 		return fmt.Errorf("failed to link output queue to audio parse: %w", err)
// 	}

// 	go func() {
// 		ticker := time.NewTicker(time.Second * 1)
// 		for {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			case <-ticker.C:
// 				state := pipeline.GetCurrentState()
// 				log.Debug(ctx, "pipeline state", "state", state)
// 			}
// 		}
// 	}()

// 	mkvsinkele, err := pipeline.GetElementByName("mkvsink")
// 	if err != nil {
// 		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
// 	}
// 	mkvsink := app.SinkFromElement(mkvsinkele)
// 	mkvsink.SetCallbacks(&app.SinkCallbacks{
// 		NewSampleFunc: WriterNewSample(ctx, w),
// 		EOSFunc: func(sink *app.Sink) {
// 			log.Warn(ctx, "mp4sink EOSFunc")
// 			cancel()
// 		},
// 	})

// 	pipeline.SetState(gst.StatePlaying)

// 	<-ctx.Done()

// 	pipeline.BlockSetState(gst.StateNull)

// 	return nil
// }
