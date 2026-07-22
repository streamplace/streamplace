package notifications

import (
	"context"
	"encoding/json"
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
				log.Error(ctx, "web push failed", "err", err)
			} else {
				success++
			}
		}(t.Token)
	}
	wg.Wait()

	log.Log(ctx, "web push blast complete", "success", success, "failed", failed, "total", len(webTargets))
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return fmt.Errorf("web push blast: %d of %d failed: %v", len(errs), len(webTargets), errs)
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
