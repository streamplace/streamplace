package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	webpush "github.com/SherClockHolmes/webpush-go"
)

// TestWebPushNotifierBlast verifies that the WebPushNotifier:
//   - ignores firebase targets,
//   - POSTs the encrypted payload to each web subscription's endpoint,
//   - surfaces a 410 Gone as an ExpiredSubscriptionError so the caller can
//     prune the dead subscription.
func TestWebPushNotifierBlast(t *testing.T) {
	// Spin up a fake push service that records what it receives.
	var (
		gotPaths  []string
		gotBodies [][]byte
		status    int
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/push/", func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		gotBodies = append(gotBodies, body)
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(201)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Generate a real VAPID keypair so the notifier can sign.
	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	notifier := NewWebPushNotifier(VAPIDKeys{PublicKey: pub, PrivateKey: priv}, "mailto:test@example.com")

	// Build a web subscription pointing at our fake push service.
	sub := webpush.Subscription{
		Endpoint: srv.URL + "/push/abc",
		Keys: webpush.Keys{
			P256dh: "BMd4Zb1d3Z2Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8Z8",
			Auth:   "dGhpcyBpcyBhbiBhdXRoIGtleQ",
		},
	}
	// Use a real, valid-length p256dh so the library doesn't reject it before
	// hitting the network. Generate a throwaway ECDH keypair for the client
	// side and use its raw public key.
	_, clientPub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	sub.Keys.P256dh = clientPub

	subJSON, err := json.Marshal(sub)
	require.NoError(t, err)

	targets := []NotificationTarget{
		{Token: "firebase-token-should-be-ignored", Type: NotificationTypeFirebase},
		{Token: string(subJSON), Type: NotificationTypeWeb},
	}

	blast := &NotificationBlast{
		Title: "🔴 @test is LIVE!",
		Body:  "hello world",
		Data:  map[string]string{"path": "/test"},
	}

	err = notifier.Blast(context.Background(), targets, blast)
	require.NoError(t, err)
	require.Len(t, gotPaths, 1, "only the web target should have been pushed to")
	require.Equal(t, "/push/abc", gotPaths[0])

	// The body is encrypted (RFC 8291), so it won't be our plaintext JSON —
	// just confirm something non-empty was sent.
	require.NotEmpty(t, gotBodies[0])

	// Now make the endpoint return 410 Gone and confirm we get an
	// ExpiredSubscriptionError.
	status = 410
	err = notifier.Blast(context.Background(), targets, blast)
	require.Error(t, err)
	var expired *ExpiredSubscriptionError
	require.ErrorAs(t, err, &expired, "410 should surface as ExpiredSubscriptionError")
	require.Equal(t, 410, expired.Status)
}

// TestMultiNotifierFanout confirms the MultiNotifier delegates to each child
// notifier and that each child only acts on its own target type.
func TestMultiNotifierFanout(t *testing.T) {
	fb := &recordingNotifier{typeFilter: NotificationTypeFirebase}
	web := &recordingNotifier{typeFilter: NotificationTypeWeb}
	multi := NewMultiNotifier(fb, web)

	targets := []NotificationTarget{
		{Token: "fb-1", Type: NotificationTypeFirebase},
		{Token: "web-1", Type: NotificationTypeWeb},
		{Token: "fb-2", Type: NotificationTypeFirebase},
	}
	blast := &NotificationBlast{Title: "t", Body: "b"}

	require.NoError(t, multi.Blast(context.Background(), targets, blast))
	require.Equal(t, []string{"fb-1", "fb-2"}, fb.seen, "firebase notifier should only see firebase targets")
	require.Equal(t, []string{"web-1"}, web.seen, "web notifier should only see web targets")
}

// TestExpiredTokensExtraction confirms that ExpiredTokens walks the
// MultiNotifier → BlastsError tree and returns the dead subscription tokens
// so callers can prune them from the DB.
func TestExpiredTokensExtraction(t *testing.T) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	notifier := NewWebPushNotifier(VAPIDKeys{PublicKey: pub, PrivateKey: priv}, "mailto:test@example.com")

	// Two web targets: one live, one that returns 410 Gone.
	var liveSub, deadSub webpush.Subscription
	_, livePub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	liveSub.Keys.P256dh = livePub
	_, deadPub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	deadSub.Keys.P256dh = deadPub

	mux := http.NewServeMux()
	liveHits := 0
	deadHits := 0
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		liveHits++
		w.WriteHeader(201)
	})
	mux.HandleFunc("/dead", func(w http.ResponseWriter, r *http.Request) {
		deadHits++
		w.WriteHeader(410)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	liveSub.Endpoint = srv.URL + "/live"
	deadSub.Endpoint = srv.URL + "/dead"
	liveJSON, _ := json.Marshal(liveSub)
	deadJSON, _ := json.Marshal(deadSub)

	targets := []NotificationTarget{
		{Token: string(liveJSON), Type: NotificationTypeWeb},
		{Token: string(deadJSON), Type: NotificationTypeWeb},
	}
	// Wrap in a MultiNotifier to test the full error tree the callers see.
	multi := NewMultiNotifier(notifier)
	err = multi.Blast(context.Background(), targets, &NotificationBlast{Title: "t", Body: "b"})
	require.Error(t, err)

	expired := ExpiredTokens(err)
	require.Len(t, expired, 1, "only the dead subscription should be extracted")
	require.Equal(t, string(deadJSON), expired[0])
	require.Equal(t, 1, liveHits)
	require.Equal(t, 1, deadHits)
}

// recordingNotifier is a test double that records the tokens it was asked to
// blast, filtered to a single type.
type recordingNotifier struct {
	typeFilter NotificationType
	seen       []string
}

func (r *recordingNotifier) Blast(ctx context.Context, targets []NotificationTarget, blast *NotificationBlast) error {
	for _, t := range targets {
		if t.Type == r.typeFilter {
			r.seen = append(r.seen, t.Token)
		}
	}
	return nil
}
