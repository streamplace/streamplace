package notifications

import (
	"context"
	"errors"
)

// MultiNotifier fans a single blast out to every transport it wraps. It
// implements Notifier by delegating each target to the child notifier that
// handles that target's Type. This is the single Notifier the rest of the
// codebase holds (replacing the old FirebaseNotifier field), so callers don't
// need to know which transports are configured.
//
// A child that returns an error for its slice of targets does not abort the
// others — each transport is independent, and a dead FCM credential
// shouldn't block web pushes (or vice versa). Errors are collected and
// returned as a joined error after all transports have been attempted.
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier wraps one or more transport notifiers. nil entries are
// skipped so callers can pass a possibly-unconfigured notifier through
// without filtering.
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	nn := &MultiNotifier{}
	for _, n := range notifiers {
		if n != nil {
			nn.notifiers = append(nn.notifiers, n)
		}
	}
	return nn
}

func (m *MultiNotifier) Blast(ctx context.Context, targets []NotificationTarget, blast *NotificationBlast) error {
	if len(m.notifiers) == 0 {
		return nil
	}
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Blast(ctx, targets, blast); err != nil {
			errs = append(errs, err)
		}
	}
	// errors.Join preserves the tree so callers can errors.As into the
	// individual transport errors (e.g. to extract expired web tokens).
	return errors.Join(errs...)
}
