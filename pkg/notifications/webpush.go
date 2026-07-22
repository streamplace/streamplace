package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
	"stream.place/streamplace/pkg/log"
)

// VAPIDKeys is the ECDSA P-256 application-server keypair Web Push requires.
// The public key is handed to the browser (it validates pushes against it);
// the private key signs the VAPID JWT on each send. Keys must stay stable —
// rotating them invalidates every existing browser subscription.
type VAPIDKeys struct {
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

// WebPushNotifier sends pushes via the Web Push protocol (RFC 8291 + VAPID).
// It implements Notifier by handling targets of NotificationTypeWeb and
// ignoring all others. Each target's Token is the JSON-serialized
// PushSubscription object the browser produced.
type WebPushNotifier struct {
	keys VAPIDKeys
	// subscriber is the contact URI embedded in the VAPID JWT (RFC 8291
	// "sub"). mailto: is conventional; a URL works too. Browsers ignore it
	// for delivery but it's required by the spec.
	subscriber string
}

// NewWebPushNotifier builds a notifier from a VAPID keypair. The subscriber
// defaults to a mailto: if empty.
func NewWebPushNotifier(keys VAPIDKeys, subscriber string) *WebPushNotifier {
	if subscriber == "" {
		subscriber = "mailto:noreply@stream.place"
	}
	return &WebPushNotifier{keys: keys, subscriber: subscriber}
}

// Blast implements Notifier. It fans a push out to every web target in
// parallel (each subscription is an independent HTTP POST to the browser
// push service). Firebase targets are ignored.
//
// Expired subscriptions (410 Gone / 404) are collected and returned as a
// *BlastError whose Expired field holds the raw subscription tokens that
// should be pruned from the DB. Callers can extract them with
// ExpiredTokens(err).
func (w *WebPushNotifier) Blast(ctx context.Context, targets []NotificationTarget, blast *NotificationBlast) error {
	webTargets := make([]NotificationTarget, 0, len(targets))
	for _, t := range targets {
		if t.Type == NotificationTypeWeb {
			webTargets = append(webTargets, t)
		}
	}
	if len(webTargets) == 0 {
		return nil
	}

	payload, err := json.Marshal(blast)
	if err != nil {
		return fmt.Errorf("error marshaling notification blast: %w", err)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
		failed  int
		errs    []error
		expired []string
	)

	for _, t := range webTargets {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			err := w.sendOne(ctx, token, payload)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				errs = append(errs, err)
				var expiredErr *ExpiredSubscriptionError
				if errors.As(err, &expiredErr) {
					expired = append(expired, token)
				}
				log.Error(ctx, "web push failed", "err", err)
			} else {
				success++
			}
		}(t.Token)
	}
	wg.Wait()

	log.Log(ctx, "web push blast complete", "success", success, "failed", failed, "total", len(webTargets), "expired", len(expired))
	if len(errs) == 0 {
		return nil
	}
	return &BlastsError{
		Errs:    errs,
		Expired: expired,
	}
}

// BlastsError wraps the per-target errors from a WebPushNotifier.Blast and
// exposes the subscription tokens that should be pruned because their
// endpoints returned 410 Gone / 404.
type BlastsError struct {
	// Errs are all per-target errors, including ExpiredSubscriptionErrors.
	Errs []error
	// Expired holds the raw subscription tokens (DB primary keys) whose
	// endpoints are dead and should be deleted.
	Expired []string
}

func (e *BlastsError) Error() string {
	if len(e.Errs) == 1 {
		return e.Errs[0].Error()
	}
	return fmt.Sprintf("web push blast: %d targets failed", len(e.Errs))
}

// Unwrap returns the wrapped errors so errors.Is / errors.As can traverse them.
func (e *BlastsError) Unwrap() []error { return e.Errs }

// ExpiredTokens walks an error tree (handling errors.Join, MultiNotifier, and
// BlastsError wrapping) and returns the raw subscription tokens whose push
// endpoints returned 410 Gone / 404. Callers should delete these rows from
// the notifications table so dead subscriptions don't accumulate.
func ExpiredTokens(err error) []string {
	if err == nil {
		return nil
	}
	var expired []string
	// If this node is a BlastsError, take its Expired slice directly and
	// stop — recursing into its Unwrap() would only re-encounter the same
	// ExpiredSubscriptionErrors and double-count.
	var be *BlastsError
	if errors.As(err, &be) {
		return be.Expired
	}
	// Otherwise traverse children (errors.Join from MultiNotifier, or a
	// single Unwrap chain) looking for nested BlastsErrors.
	for _, inner := range errorsUnwrap(err) {
		expired = append(expired, ExpiredTokens(inner)...)
	}
	return expired
}

// errorsUnwrap returns the direct children of err for tree-walking. Supports
// errors.Join (Unwrap() []error) and single-wrap (Unwrap() error).
func errorsUnwrap(err error) []error {
	// errors.Join / BlastsError expose Unwrap() []error
	type multiUnwrapper interface{ Unwrap() []error }
	if mu, ok := err.(multiUnwrapper); ok {
		return mu.Unwrap()
	}
	// standard single-wrap
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		if inner := u.Unwrap(); inner != nil {
			return []error{inner}
		}
	}
	return nil
}

// sendOne decrypts the stored subscription JSON and POSTs the encrypted
// payload to the browser's push endpoint.
func (w *WebPushNotifier) sendOne(ctx context.Context, subscriptionJSON string, payload []byte) error {
	var sub webpush.Subscription
	if err := json.Unmarshal([]byte(subscriptionJSON), &sub); err != nil {
		return fmt.Errorf("error parsing web push subscription: %w", err)
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &sub, &webpush.Options{
		VAPIDPublicKey:  w.keys.PublicKey,
		VAPIDPrivateKey: w.keys.PrivateKey,
		Subscriber:      w.subscriber,
		TTL:             24 * 60 * 60, // 24h
	})
	if err != nil {
		return fmt.Errorf("error sending web push: %w", err)
	}
	defer resp.Body.Close()
	// 410 Gone means the subscription is no longer valid; the caller should
	// prune it. We surface it distinctly so the queue processor can react.
	if resp.StatusCode == 410 || resp.StatusCode == 404 {
		return &ExpiredSubscriptionError{Endpoint: sub.Endpoint, Status: resp.StatusCode}
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("web push endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

// ExpiredSubscriptionError indicates a push endpoint returned 410 Gone (or
// 404), meaning the subscription is dead and should be removed from the DB.
type ExpiredSubscriptionError struct {
	Endpoint string
	Status   int
}

func (e *ExpiredSubscriptionError) Error() string {
	return fmt.Sprintf("web push subscription expired (status %d): %s", e.Status, e.Endpoint)
}
