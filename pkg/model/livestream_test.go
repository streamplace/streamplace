package model

import (
	"testing"
)

// TestToLivestreamViewNil verifies ToLivestreamView returns an error instead of
// panicking when called on a nil receiver or when the embedded Livestream blob
// is nil. Reproduces the panic reported in
// https://github.com/streamplace/streamplace/issues/1079.
func TestToLivestreamViewNil(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var ls *Livestream
		view, err := ls.ToLivestreamView()
		if err == nil {
			t.Fatalf("expected error for nil receiver, got nil")
		}
		if view != nil {
			t.Fatalf("expected nil view for nil receiver, got %+v", view)
		}
	})

	t.Run("nil livestream blob", func(t *testing.T) {
		ls := &Livestream{}
		view, err := ls.ToLivestreamView()
		if err == nil {
			t.Fatalf("expected error for nil livestream blob, got nil")
		}
		if view != nil {
			t.Fatalf("expected nil view for nil livestream blob, got %+v", view)
		}
	})
}
