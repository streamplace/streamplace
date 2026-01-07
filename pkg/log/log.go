/*
Package clog provides Context with logging metadata, as well as logging helper functions.
*/
package log

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"

	"github.com/bluesky-social/indigo/util"
	"github.com/golang/glog"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
)

// unique type to prevent assignment.
type clogContextKeyType struct{}

// singleton value to identify our logging metadata in context
var clogContextKey = clogContextKeyType{}

// unique type to prevent assignment.
type clogDebugKeyType struct{}

// singleton value to identify our debug cli flag
var clogDebugKey = clogDebugKeyType{}

var errorLogLevel glog.Level = 1
var warnLogLevel glog.Level = 2
var defaultLogLevel glog.Level = 3
var debugLogLevel glog.Level = 4
var traceLogLevel glog.Level = 9

// basic type to represent logging container. logging context is immutable after
// creation, so we don't have to worry about locking.
type metadata [][]string

// LOG: I0809 17:23:01.775710  255904 broadcast.go:1162] manifestID=didplcdkh4rwafdcda4ko7lewe43ml-84zq1aff-4ren nonce=9899027318953397582 seqNo=2 clientIP=127.0.0.1 Trying to transcode segment using sessions=1
// LOG: I0809 17:23:01.777792  255904 segment_rpc.go:551] manifestID=didplcdkh4rwafdcda4ko7lewe43ml-84zq1aff-4ren nonce=9899027318953397582 seqNo=2 orchSessionID=cdc9e3b9 orchestrator=https://127.0.0.1:9001 clientIP=127.0.0.1 Submitting segment bytes=1017268 orch=https://127.0.0.1:9001 timeout=8s uploadTimeout=2s segDur=0.832
// LOG: I0809 17:23:01.778444  255904 discovery.go:211] manifestID=didplcdkh4rwafdcda4ko7lewe43ml-84zq1aff-4ren nonce=9899027318953397582 clientIP=127.0.0.1 Done fetching orch info numOrch=1 responses=1/1 timedOut=false
// LOG: I0809 17:23:01.782914  255904 segment_rpc.go:587] manifestID=didplcdkh4rwafdcda4ko7lewe43ml-84zq1aff-4ren nonce=9899027318953397582 seqNo=2 orchSessionID=cdc9e3b9 orchestrator=https://127.0.0.1:9001 clientIP=127.0.0.1 Uploaded segment orch=https://127.0.0.1:9001 dur=5.106541ms
// LOG: I0809 17:23:01.892735  255904 segment_rpc.go:668] manifestID=didplcdkh4rwafdcda4ko7lewe43ml-84zq1aff-4ren nonce=9899027318953397582 seqNo=2 orchSessionID=cdc9e3b9 orchestrator=https://127.0.0.1:9001 clientIP=127.0.0.1 Successfully transcoded segment segName= seqNo=2 orch=https://127.0.0.1:9001 dur=109.762457ms
// LOG: I0809 17:23:01.893821  255904 mediaserver.go:1046] manifestID=didplcdkh4rwafdcda4ko7lewe43ml-84zq1aff-4ren nonce=9899027318953397582 seqNo=2 orchSessionID=cdc9e3b9 orchestrator=https://127.0.0.1:9001 clientIP=127.0.0.1 Finished transcoding push request at url=http://127.0.0.1:8935/live/didplcdkh4rwafdcda4ko7lewe43ml-84zq1aff-4ren/2.ts took=160.894832ms

var realStderr = os.Stderr
var logLineRegex = regexp.MustCompile(`^([IWEF])\d+ \d{2}:\d{2}:\d{2}\.\d{6}\s+\d+\s+([^:]+:\d+)\]\s*(.*)$`)

func SetColorLogger(color string) {
	noColor := false
	if color == "true" {
		noColor = false
	} else if color == "false" {
		noColor = true
	} else {
		noColor = !isatty.IsTerminal(realStderr.Fd())
	}
	// set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(realStderr, &tint.Options{
			Level: slog.LevelDebug,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key != "time" {
					return a
				}
				t := a.Value.Time().UTC()
				return slog.Attr{
					Key:   "time",
					Value: slog.TimeValue(t),
				}
			},
			TimeFormat: util.ISO8601,
			NoColor:    noColor,
		}),
	))
}

func MonkeypatchStderr() {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	realStderr := os.Stderr
	os.Stderr = w
	ctx := WithLogValues(context.Background(), "component", "livepeer")
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			match := logLineRegex.FindStringSubmatch(scanner.Text())
			if len(match) == 0 {
				fmt.Fprintf(realStderr, "%s\n", scanner.Text())
				continue
			}
			level := match[1]
			caller := match[2]
			message := match[3]
			if level == "I" {
				Log(ctx, message, "caller", caller)
			} else if level == "W" {
				Warn(ctx, message)
			} else if level == "E" {
				Error(ctx, message)
			} else if level == "F" {
				Warn(ctx, message)
			} else {
				fmt.Fprintf(realStderr, "UNKNOWN LOG LEVEL: %s %s\n", level, message)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(realStderr, "LOG: Error reading pipe: %v\n", err)
		}

	}()
	// set global logger with custom options
	SetColorLogger("")

	// Set default v level to 3; this is overridden in main() but is useful for tests
	vFlag := flag.Lookup("v")
	// nolint:errcheck
	vFlag.Value.Set(fmt.Sprintf("%d", defaultLogLevel))
}

type VerboseLogger struct {
	level glog.Level
}

// implementation of our logger aware of glog -v=[0-9] levels
func V(level glog.Level) *VerboseLogger {
	return &VerboseLogger{level: level}
}

func (m metadata) Map() map[string]string {
	out := map[string]string{}
	for _, pair := range m {
		out[pair[0]] = pair[1]
	}
	return out
}

func (m metadata) Flat() []any {
	out := []any{}
	for _, pair := range m {
		out = append(out, pair[0])
		out = append(out, pair[1])
	}
	return out
}

// Return a new context, adding in the provided values to the logging metadata
func WithLogValues(ctx context.Context, args ...string) context.Context {
	oldMetadata, _ := ctx.Value(clogContextKey).(metadata)
	// No previous logging found, set up a new map
	if oldMetadata == nil {
		oldMetadata = metadata{}
	}
	var newMetadata = metadata{}
	for _, pair := range oldMetadata {
		newMetadata = append(newMetadata, []string{pair[0], pair[1]})
	}
	for i := range args {
		if i%2 == 0 {
			continue
		}
		newKey := args[i-1]
		newValue := args[i]
		found := false
		for _, pair := range newMetadata {
			if pair[0] == newKey {
				pair[1] = newValue
				found = true
				break
			}
		}
		if !found {
			newMetadata = append(newMetadata, []string{newKey, newValue})
		}
	}
	return context.WithValue(ctx, clogContextKey, newMetadata)
}

// Return a new context, adding in the provided values to the logging metadata
func WithDebugValue(ctx context.Context, debug map[string]map[string]int) context.Context {
	return context.WithValue(ctx, clogDebugKey, debug)
}

// Actual log handler; the others have wrappers to properly handle stack depth
func (v *VerboseLogger) log(ctx context.Context, message string, fn func(string, ...any), args ...any) {
	// I want a compile time assertion for this... but short of that let's be REALLY ANNOYING
	if len(args)%2 != 0 {
		for range 6 {
			fmt.Println("!!!!!!!!!!!!!!!! FOLLOWING LOG LINE HAS AN ODD NUMBER OF ARGUMENTS !!!!!!!!!!!!!!!!")
		}
	}
	meta, metaOk := ctx.Value(clogContextKey).(metadata)
	found := false
	highestLevel := glog.Level(0)
	debug, debugOk := ctx.Value(clogDebugKey).(map[string]map[string]int)

	// debug is {"func": {"ToHLS": 3}, "file": {"gstreamer.go": 4}}
	// meta is {"func": "ToHLS", "file": "gstreamer.go"}
	// we want to use the highest level between debug and meta
	if debugOk && metaOk {
		for mk, mv := range meta.Map() {
			debugValuesForMetaValue, ok := debug[mk]
			if !ok {
				continue
			}
			ll, ok := debugValuesForMetaValue[mv]
			if !ok {
				continue
			}
			if glog.Level(ll) > highestLevel {
				found = true
				highestLevel = glog.Level(ll)
			}
		}
	}
	if found {
		if v.level > highestLevel {
			return
		}
	} else {
		if !glog.V(v.level) {
			return
		}
	}

	hasCaller := false

	allArgs := []any{}
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, meta.Flat()...)
	for i := range args {
		if i%2 == 0 {
			continue
		}
		if args[i-1] == "caller" {
			hasCaller = true
		}
	}
	if !hasCaller {
		allArgs = append(allArgs, "caller", caller(3))
	}

	fn(message, allArgs...)
}

func (v *VerboseLogger) Log(ctx context.Context, message string, args ...any) {
	if v.level >= 4 {
		v.log(ctx, message, slog.Debug, args...)
	} else {
		v.log(ctx, message, slog.Info, args...)
	}
}

func Error(ctx context.Context, message string, args ...any) {
	V(errorLogLevel).log(ctx, message, slog.Error, args...)
}

func Warn(ctx context.Context, message string, args ...any) {
	V(warnLogLevel).log(ctx, message, slog.Warn, args...)
}

func Log(ctx context.Context, message string, args ...any) {
	V(defaultLogLevel).log(ctx, message, slog.Info, args...)
}

func Debug(ctx context.Context, message string, args ...any) {
	V(debugLogLevel).log(ctx, message, slog.Info, args...)
}

func Trace(ctx context.Context, message string, args ...any) {
	V(traceLogLevel).log(ctx, message, slog.Info, args...)
}

// returns true if we are at least the given level
func Level(level glog.Level) glog.Verbose {
	return glog.V(level)
}

func GetValue(ctx context.Context, key string) string {
	meta, metaOk := ctx.Value(clogContextKey).(metadata)
	if !metaOk {
		return ""
	}
	return meta.Map()[key]
}

// returns filenames relative to streamplace root
// e.g. handlers/misttriggers/triggers.go:58
func caller(depth int) string {
	_, myfile, _, _ := runtime.Caller(0)
	// This assumes that the root directory of streamplace is two levels above this folder.
	// If that changes, please update this rootDir resolution.
	rootDir := filepath.Join(filepath.Dir(myfile), "..", "..")
	_, file, line, _ := runtime.Caller(depth)
	rel, _ := filepath.Rel(rootDir, file)
	return rel + ":" + strconv.Itoa(line)
}
