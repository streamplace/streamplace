package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
)

// Demonstration of a GStreamer (or go-gst idk) leak

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("expected 1 argument")
	}
	os.Setenv("GST_DEBUG", "leaks:9,GST_TRACER:9")
	os.Setenv("GST_TRACERS", "leaks")
	os.Setenv("GST_LEAKS_TRACER_SIG", "1")
	err := RunPipeline(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	runtime.GC()
	time.Sleep(1 * time.Second)
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		log.Fatal(err)
	}
	process.Signal(syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
}

func RunPipeline(file string) error {
	gst.Init(nil)
	pipeline, err := gst.NewPipelineFromString(fmt.Sprintf("filesrc location=%s ! qtdemux ! fakesink", file))
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)
	pipeline.SetState(gst.StatePlaying)

	pipeline.GetBus().AddWatch(func(msg *gst.Message) bool {
		if msg.Type() == gst.MessageEOS {
			mainLoop.Quit()
			return false
		}
		return true
	})

	mainLoop.Run()

	pipeline.BlockSetState(gst.StateNull)

	return nil
}
