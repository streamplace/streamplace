package notifications

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
	"stream.place/streamplace/pkg/log"
)

type FirebaseNotifier interface {
	Blast(ctx context.Context, tokens []string, golive *NotificationBlast) error
}

type FirebaseNotifierS struct {
	app *firebase.App
}

type GoogleCredential struct {
	ProjectID string `json:"project_id"`
}

type NotificationBlast struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data"`
}

func MakeFirebaseNotifier(ctx context.Context, serviceAccountJSONb64 string) (FirebaseNotifier, error) {
	// string can optionally be base64-encoded
	serviceAccountJSON := serviceAccountJSONb64
	dec, err := base64.StdEncoding.DecodeString(serviceAccountJSONb64)
	if err == nil {
		// succeeded, cool! use that.
		serviceAccountJSON = string(dec)
	}
	var cred GoogleCredential
	err = json.Unmarshal([]byte(serviceAccountJSON), &cred)
	if err != nil {
		return nil, fmt.Errorf("error trying to discover project_id: %w", err)
	}
	conf := &firebase.Config{
		ProjectID: cred.ProjectID,
	}
	opt := option.WithCredentialsJSON([]byte(serviceAccountJSON))
	app, err := firebase.NewApp(ctx, conf, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}
	return &FirebaseNotifierS{app: app}, nil
}

// refactor me when we have >500 users
func (f *FirebaseNotifierS) Blast(ctx context.Context, tokens []string, blast *NotificationBlast) error {
	client, err := f.app.Messaging(ctx)
	if err != nil {
		return err
	}

	notification := &messaging.MulticastMessage{
		Tokens: tokens,
		Data:   blast.Data,
		Notification: &messaging.Notification{
			Title: blast.Title,
			Body:  blast.Body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound: "default",
			},
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
		},
	}
	res, err := client.SendEachForMulticast(ctx, notification)
	if err != nil {
		return err
	}
	log.Log(ctx, "notification blast successful", "successCount", res.SuccessCount, "failureCount", res.FailureCount)
	return nil
}
