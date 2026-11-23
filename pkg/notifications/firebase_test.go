package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

var fakeServiceAccount = `
{
  "type": "service_account",
  "private_key_id": "abc",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDY3E8o1NEFcjMM\nHW/5ZfFJw29/8NEqpViNjQIx95Xx5KDtJ+nWn9+OW0uqsSqKlKGhAdAo+Q6bjx2c\nuXVsXTu7XrZUY5Kltvj94DvUa1wjNXs606r/RxWTJ58bfdC+gLLxBfGnB6CwK0YQ\nxnfpjNbkUfVVzO0MQD7UP0Hl5ZcY0Puvxd/yHuONQn/rIAieTHH1pqgW+zrH/y3c\n59IGThC9PPtugI9ea8RSnVj3PWz1bX2UkCDpy9IRh9LzJLaYYX9RUd7++dULUlat\nAaXBh1U6emUDzhrIsgApjDVtimOPbmQWmX1S60mqQikRpVYZ8u+NDD+LNw+/Eovn\nxCj2Y3z1AgMBAAECggEAWDBzoqO1IvVXjBA2lqId10T6hXmN3j1ifyH+aAqK+FVl\nGjyWjDj0xWQcJ9ync7bQ6fSeTeNGzP0M6kzDU1+w6FgyZqwdmXWI2VmEizRjwk+/\n/uLQUcL7I55Dxn7KUoZs/rZPmQDxmGLoue60Gg6z3yLzVcKiDc7cnhzhdBgDc8vd\nQorNAlqGPRnm3EqKQ6VQp6fyQmCAxrr45kspRXNLddat3AMsuqImDkqGKBmF3Q1y\nxWGe81LphUiRqvqbyUlh6cdSZ8pLBpc9m0c3qWPKs9paqBIvgUPlvOZMqec6x4S6\nChbdkkTRLnbsRr0Yg/nDeEPlkhRBhasXpxpMUBgPywKBgQDs2axNkFjbU94uXvd5\nznUhDVxPFBuxyUHtsJNqW4p/ujLNimGet5E/YthCnQeC2P3Ym7c3fiz68amM6hiA\nOnW7HYPZ+jKFnefpAtjyOOs46AkftEg07T9XjwWNPt8+8l0DYawPoJgbM5iE0L2O\nx8TU1Vs4mXc+ql9F90GzI0x3VwKBgQDqZOOqWw3hTnNT07Ixqnmd3dugV9S7eW6o\nU9OoUgJB4rYTpG+yFqNqbRT8bkx37iKBMEReppqonOqGm4wtuRR6LSLlgcIU9Iwx\nyfH12UWqVmFSHsgZFqM/cK3wGev38h1WBIOx3/djKn7BdlKVh8kWyx6uC8bmV+E6\nOoK0vJD6kwKBgHAySOnROBZlqzkiKW8c+uU2VATtzJSydrWm0J4wUPJifNBa/hVW\ndcqmAzXC9xznt5AVa3wxHBOfyKaE+ig8CSsjNyNZ3vbmr0X04FoV1m91k2TeXNod\njMTobkPThaNm4eLJMN2SQJuaHGTGERWC0l3T18t+/zrDMDCPiSLX1NAvAoGBAN1T\nVLJYdjvIMxf1bm59VYcepbK7HLHFkRq6xMJMZbtG0ryraZjUzYvB4q4VjHk2UDiC\nlhx13tXWDZH7MJtABzjyg+AI7XWSEQs2cBXACos0M4Myc6lU+eL+iA+OuoUOhmrh\nqmT8YYGu76/IBWUSqWuvcpHPpwl7871i4Ga/I3qnAoGBANNkKAcMoeAbJQK7a/Rn\nwPEJB+dPgNDIaboAsh1nZhVhN5cvdvCWuEYgOGCPQLYQF0zmTLcM+sVxOYgfy8mV\nfbNgPgsP5xmu6dw2COBKdtozw0HrWSRjACd1N4yGu75+wPCcX/gQarcjRcXXZeEa\nNtBLSfcqPULqD+h7br9lEJio\n-----END PRIVATE KEY-----\n",
  "client_email": "123-abc@developer.gserviceaccount.com",
  "client_id": "123-abc.apps.googleusercontent.com",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
	"project_id": "fakeproject",
	"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/firebase-adminsdk-fmlii%40fakeproject.iam.gserviceaccount.com",
	"universe_domain": "googleapis.com"
}
`

func TestFirebaseNotifier(t *testing.T) {
	_, err := MakeFirebaseNotifier(context.Background(), fakeServiceAccount)
	require.NoError(t, err)
}

func TestFirebasePushDelivery(t *testing.T) {
	type message struct {
		Token        string                  `json:"token"`
		Notification messaging.Notification  `json:"notification"`
		Data         map[string]string       `json:"data"`
		Android      messaging.AndroidConfig `json:"android"`
		APNS         messaging.APNSConfig    `json:"apns"`
	}
	type delivery struct {
		message message
		err     error
	}
	inbox := make(chan delivery, 2)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message message `json:"message"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		inbox <- delivery{message: body.Message, err: err}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"projects/test/messages/1"}`))
	}))
	t.Cleanup(gateway.Close)

	app, err := firebase.NewApp(t.Context(), &firebase.Config{ProjectID: "test"},
		option.WithEndpoint(gateway.URL), option.WithHTTPClient(gateway.Client()))
	require.NoError(t, err)
	notifier := &FirebaseNotifierS{app: app}
	blast := &NotificationBlast{
		Title: "First livestream from @streamer.test",
		Body:  "Debut",
		Data:  map[string]string{"path": "/streamer.test"},
	}
	require.NoError(t, notifier.Blast(t.Context(), []NotificationTarget{
		{Token: "native-device", Type: NotificationTypeFirebase},
		{Token: "web-subscription", Type: NotificationTypeWeb},
	}, blast))
	require.Len(t, inbox, 1)
	got := <-inbox
	require.NoError(t, got.err)
	require.Equal(t, "native-device", got.message.Token)
	require.Equal(t, blast.Title, got.message.Notification.Title)
	require.Equal(t, blast.Body, got.message.Notification.Body)
	require.Equal(t, blast.Data, got.message.Data)
	require.Equal(t, "default", got.message.Android.Notification.Sound)
	require.Equal(t, "default", got.message.APNS.Payload.Aps.Sound)
}
