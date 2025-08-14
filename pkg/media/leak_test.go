package media

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/cenkalti/backoff/v5"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
)

var streamplaceTestCount = 50

func init() {
	testRunsStr := os.Getenv("STREAMPLACE_TEST_COUNT")
	if testRunsStr != "" {
		var err error
		streamplaceTestCount, err = strconv.Atoi(testRunsStr)
		if err != nil {
			panic(fmt.Sprintf("STREAMPLACE_TEST_COUNT is not a number: %s", testRunsStr))
		}
	}
}

var LeakTestMutex sync.Mutex

const IgnoreLeaks = "STREAMPLACE_IGNORE_LEAKS"
const ShowTrace = "STREAMPLACE_SHOW_TRACE"
const GSTDebugNeeded = "leaks:9,GST_TRACER:9"
const LeakLine = "GST_TRACER :0:: object-alive"

var LeakDoneRegex = regexp.MustCompile(`listed\s+(\d+)\s+alive\s+objects`)

var LeakReport = []string{}
var LeakReportMutex sync.Mutex
var LeakDoneCh = make(chan struct{})

func TestMain(m *testing.M) {
	if os.Getenv(IgnoreLeaks) != "" {
		gstinit.InitGST()
		os.Exit(m.Run())
		return
	}
	showTrace := os.Getenv(ShowTrace) != ""
	gstDebug := os.Getenv("GST_DEBUG")
	if gstDebug == "" {
		gstDebug = GSTDebugNeeded
	} else {
		gstDebug = fmt.Sprintf("%s,%s", gstDebug, GSTDebugNeeded)
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
			lineAnsi := scanner.Text()
			line := stripansi.Strip(lineAnsi)
			if !strings.Contains(line, " TRACE ") || showTrace {
				fmt.Println(lineAnsi)
			}
			if strings.Contains(line, LeakLine) {
				LeakReportMutex.Lock()
				LeakReport = append(LeakReport, line)
				LeakReportMutex.Unlock()
			} else if LeakDoneRegex.MatchString(line) {
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

// Often the GC is instance, but sometimes it takes a while. So, we retry a few times
// with exponential backoff, giving the GC more time to do its thing.
func getLeakCount(t *testing.T) int {
	ticker := backoff.NewTicker(backoff.NewExponentialBackOff())
	defer ticker.Stop()
	var leaks int
	for i := 0; i < 10; i++ {
		leaks = getLeakCountInner(t)
		if leaks == 0 {
			return leaks
		}
		if i < 9 {
			<-ticker.C
		}
	}
	return leaks
}

func getLeakCountInner(t *testing.T) int {
	if os.Getenv(IgnoreLeaks) != "" {
		return 0
	}
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)

	LeakReportMutex.Lock()
	LeakReport = []string{}
	LeakReportMutex.Unlock()

	// we want CI to be extra reliable here and a little slower is okay
	flushes := 2

	for range flushes {
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
			for !done {

				runtime.GC()
				runtime.GC()
				time.Sleep(500 * time.Millisecond)
			}
			<-ch
		}()
	}

	err = process.Signal(os.Signal(syscall.SIGUSR1))
	require.NoError(t, err)

	<-LeakDoneCh

	LeakReportMutex.Lock()
	after := len(LeakReport)
	LeakReportMutex.Unlock()
	return after
}

func checkGStreamerLeaks(t *testing.T, expected int) {
	if os.Getenv(IgnoreLeaks) != "" {
		return
	}
	leaks := getLeakCount(t)
	if leaks > expected {
		LeakReportMutex.Lock()
		for _, l := range LeakReport {
			fmt.Println(l)
		}
		LeakReportMutex.Unlock()
		require.Equal(t, expected, len(LeakReport), "Leaks found")
	}
}

func withNoGSTLeaks(t *testing.T, f func()) {
	LeakTestMutex.Lock()
	defer LeakTestMutex.Unlock()
	gstinit.InitGST()
	before := getLeakCount(t)
	defer checkGStreamerLeaks(t, before)
	// ignore := goleak.IgnoreCurrent()
	// defer goleak.VerifyNone(t, ignore)
	f()
}
