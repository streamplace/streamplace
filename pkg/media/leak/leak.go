package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/go-gst/go-glib/pkg/glib/v2"
	"github.com/go-gst/go-gst/pkg/gst"
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
	if err := process.Signal(syscall.SIGUSR1); err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)
}

func RunPipeline(file string) error {
	gst.Init()
	element, err := gst.ParseLaunch(fmt.Sprintf("filesrc location=%s ! qtdemux ! fakesink", file))
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}
	pipeline := element.(gst.Pipeline)

	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)
	if status := pipeline.BlockSetState(gst.StatePlaying, gst.ClockTimeNone); status != gst.StateChangeSuccess {
		return fmt.Errorf("failed to set pipeline state: %s", status)
	}

	pipeline.GetBus().AddWatchFull(0, func(_ gst.Bus, msg *gst.Message) bool {
		if msg.Type() == gst.MessageEOS {
			mainLoop.Quit()
			return false
		}
		return true
	})

	mainLoop.Run()

	if status := pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone); status != gst.StateChangeSuccess {
		return fmt.Errorf("failed to set pipeline state: %s", status)
	}

	return nil
}
