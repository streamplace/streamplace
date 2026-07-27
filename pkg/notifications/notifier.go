package notifications

import "context"

// NotificationType identifies the push transport a token belongs to. It lives
// in pkg/notifications (the lower-level package) so that pkg/statedb — which
// already imports pkg/notifications for the Notifier field — can reference it
// on the Notification row without creating a circular import.
type NotificationType string

const (
	// NotificationTypeFirebase is an FCM/APNs registration token (mobile).
	NotificationTypeFirebase NotificationType = "firebase"
	// NotificationTypeWeb is a Web Push subscription (endpoint + p256dh/auth
	// keys), stored as the JSON-serialized PushSubscription object.
	NotificationTypeWeb NotificationType = "web"
)

// NotificationTarget pairs a push token with the transport that knows how to
// deliver to it. Blast callers produce a []NotificationTarget (typically by
// loading notification rows from the DB) and hand them to a Notifier, which
// fans each target out to the matching transport.
type NotificationTarget struct {
	Token string
	Type  NotificationType
}

// Notifier sends a notification blast to a set of targets. Each
// implementation handles one transport (firebase, web) or fans out across
// several (MultiNotifier). Implementations should silently skip targets whose
// Type they don't handle.
type Notifier interface {
	Blast(ctx context.Context, targets []NotificationTarget, blast *NotificationBlast) error
}
