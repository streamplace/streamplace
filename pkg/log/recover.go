package log

import (
	"context"
	"fmt"
	"runtime/debug"
)

// Recover wraps a function call so that a panic is recovered, logged at error
// level with the provided context, and returned as an error rather than
// propagating and crashing the calling goroutine.
//
// This is defense-in-depth for long-lived goroutines (the firehose event loop,
// the task queue workers) so that one bad event or task can't take down the
// whole node. Callers that already return an error should prefer wrapping
// their body in this to convert panics into logged errors; callers that don't
// care about the returned error can ignore it.
//
// The recovered value is returned as a PanicError so callers can wrap/join it
// like any error. The goroutine's stack is captured and logged to aid triage.
func Recover(ctx context.Context, fn func() error) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		stack := debug.Stack()
		Error(ctx, "recovered panic", "panic", r, "stack", string(stack))
		err = &PanicError{Recovered: r, Stack: stack}
	}()
	return fn()
}

// RecoverVoid is like Recover but for functions that do not return an error.
// It recovers and logs a panic without propagating it, so a panicking side
// effect can't crash the calling goroutine. Use it when there is nothing
// meaningful to do with a returned error anyway.
func RecoverVoid(ctx context.Context, fn func()) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		stack := debug.Stack()
		Error(ctx, "recovered panic", "panic", r, "stack", string(stack))
	}()
	fn()
}

// PanicError wraps a value recovered from a panic. It implements the error
// interface so it can flow through normal error-handling paths.
type PanicError struct {
	Recovered any
	Stack     []byte
}

func (e *PanicError) Error() string {
	return "panic recovered: " + asString(e.Recovered)
}

// Unwrap is intentionally a no-op; a panic has no sentinel error to unwrap
// into, but defining it keeps linters quiet about the field.
func (e *PanicError) Unwrap() error { return nil }

// asString stringifies the recovered value, tolerating non-string panic
// values (e.g. runtime errors, ints).
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return fmt.Sprintf("%v", v)
}
