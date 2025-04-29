package media

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
)

const IGNORE_LEAKS = "STREAMPLACE_IGNORE_LEAKS"
const GST_DEBUG_NEEDED = "leaks:9,GST_TRACER:9"
const LEAK_LINE = "GST_TRACER :0:: object-alive"

var LEAK_DONE_REGEX = regexp.MustCompile(`listed\s+(\d+)\s+alive\s+objects`)

var LeakReport = []string{}
var LeakReportMutex sync.Mutex
var LeakDoneCh = make(chan struct{})

func TestMain(m *testing.M) {
	if os.Getenv(IGNORE_LEAKS) != "" {
		gstinit.InitGST()
		os.Exit(m.Run())
		return
	}
	gstDebug := os.Getenv("GST_DEBUG")
	if gstDebug == "" {
		gstDebug = GST_DEBUG_NEEDED
	} else {
		gstDebug = fmt.Sprintf("%s,%s", gstDebug, GST_DEBUG_NEEDED)
	}
	os.Setenv("GST_DEBUG", gstDebug)
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
			fmt.Println(line)
			line = stripansi.Strip(line)
			if strings.Contains(line, LEAK_LINE) {
				LeakReportMutex.Lock()
				LeakReport = append(LeakReport, line)
				LeakReportMutex.Unlock()
			} else if LEAK_DONE_REGEX.MatchString(line) {
				LeakDoneCh <- struct{}{}
			} else {
				continue
			}
		}
		if err := scanner.Err(); err != nil {
			panic(err)
		}
	}()
	gstinit.InitGST()
	os.Exit(m.Run())
}

func getLeakCount(t *testing.T) int {
	if os.Getenv(IGNORE_LEAKS) != "" {
		return 0
	}
	process, err := os.FindProcess(os.Getpid())
	LeakReportMutex.Lock()
	LeakReport = []string{}
	LeakReportMutex.Unlock()

	// we want CI to be extra reliable here and a little slower is okay
	flushes := 2
	if os.Getenv("CI") != "" {
		flushes = 5
	}

	for i := 0; i < flushes; i++ {
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
			<-ch
		}()
	}

	time.Sleep(time.Duration(flushes) * time.Second)

	err = process.Signal(os.Signal(syscall.SIGUSR1))
	require.NoError(t, err)

	<-LeakDoneCh

	LeakReportMutex.Lock()
	after := len(LeakReport)
	LeakReportMutex.Unlock()
	return after
}

func checkGStreamerLeaks(t *testing.T, expected int) {
	if os.Getenv(IGNORE_LEAKS) != "" {
		return
	}
	leaks := getLeakCount(t)
	if leaks > expected {
		LeakReportMutex.Lock()
		for _, l := range LeakReport {
			fmt.Println(l)
		}
		LeakReportMutex.Unlock()
	}
	require.Equal(t, expected, len(LeakReport), "Leaks found")
}
