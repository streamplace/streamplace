package log

import (
	"context"
	"errors"
	"testing"
)

func TestRecoverReturnsErrorFromPanic(t *testing.T) {
	ctx := context.Background()
	err := Recover(ctx, func() error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected an error from a panicked call, got nil")
	}
	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected a *PanicError, got %T: %v", err, err)
	}
	if pe.Recovered != "boom" {
		t.Fatalf("expected Recovered to be %q, got %v", "boom", pe.Recovered)
	}
	if len(pe.Stack) == 0 {
		t.Fatal("expected a non-empty captured stack")
	}
	if got := err.Error(); got != "panic recovered: boom" {
		t.Fatalf("unexpected Error(): %q", got)
	}
}

func TestRecoverPassesThroughNormalError(t *testing.T) {
	ctx := context.Background()
	want := errors.New("ordinary failure")
	err := Recover(ctx, func() error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected the wrapped normal error to be returned, got %v", err)
	}
}

func TestRecoverReturnsNilOnSuccess(t *testing.T) {
	ctx := context.Background()
	err := Recover(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil on a successful call, got %v", err)
	}
}

func TestRecoverHandlesNonStringPanic(t *testing.T) {
	ctx := context.Background()
	err := Recover(ctx, func() error {
		panic(42)
	})
	if err == nil {
		t.Fatal("expected an error from an int panic, got nil")
	}
	if got := err.Error(); got != "panic recovered: 42" {
		t.Fatalf("expected stringified int panic, got %q", got)
	}
}

func TestRecoverVoidDoesNotPropagatePanic(t *testing.T) {
	ctx := context.Background()
	// If RecoverVoid fails to recover, the panic crashes the test process.
	RecoverVoid(ctx, func() {
		panic("must not escape")
	})
}
