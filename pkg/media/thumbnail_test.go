package media

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/gstinit"
)

const LEAK_LINE = "GST_TRACER :0:: object-alive"

var LeakReport = []string{}
var LeakReportMutex sync.Mutex

func TestMain(m *testing.M) {
	os.Setenv("GST_DEBUG", "GST_TRACER:7")
	os.Setenv("GST_TRACERS", "leaks")
	os.Setenv("GST_LEAKS_TRACER_SIG", "1")
	debug.SetGCPercent(5)

	f, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
	fName := filepath.Join(f, "leak.log")
	err = syscall.Mkfifo(fName, 0640)
	if err != nil {
		panic(err)
	}
	os.Setenv("GST_DEBUG_FILE", fName)

	go func() {
		pipe, err := os.OpenFile(fName, os.O_RDONLY, 0640)
		if err != nil {
			panic(err)
		}
		defer pipe.Close()
		// Read and print each line from FD
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			line = stripansi.Strip(line)
			if !strings.Contains(line, LEAK_LINE) {
				continue
			}
			LeakReportMutex.Lock()
			LeakReport = append(LeakReport, line)
			LeakReportMutex.Unlock()
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading from file: %v", err)
		}
	}()
	os.Exit(m.Run())
}

func checkGStreamerLeaks(t *testing.T) {
	LeakReportMutex.Lock()
	LeakReport = []string{}
	LeakReportMutex.Unlock()
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)

	ch := make(chan struct{})
	done := false

	go func() {
		thing := &[]byte{}
		runtime.SetFinalizer(thing, func(thing *[]byte) {
			done = true
			ch <- struct{}{}
		})
	}()

	go func() {
		runtime.GC()
		runtime.GC()
		for {
			if done {
				break
			}
			runtime.GC()
			runtime.GC()
			time.Sleep(500 * time.Millisecond)
		}
	}()

	<-ch
	time.Sleep(1 * time.Second)

	err = process.Signal(os.Signal(syscall.SIGUSR1))
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	LeakReportMutex.Lock()
	for _, l := range LeakReport {
		fmt.Println(l)
	}
	require.Equal(t, 0, len(LeakReport), "Leaks found")
	LeakReportMutex.Unlock()
}

func TestThumbnail(t *testing.T) {
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)
	gstinit.InitGST()

	// Open input file
	inputFile, err := os.Open(getFixture("sample-segment.mp4"))
	require.NoError(t, err)
	defer inputFile.Close()

	thumbnail := bytes.Buffer{}
	err = Thumbnail(context.Background(), inputFile, &thumbnail)
	require.NoError(t, err)
	require.NotNil(t, thumbnail)
	require.Greater(t, thumbnail.Len(), 0, "Thumbnail buffer should not be empty")

	checkGStreamerLeaks(t)
}
