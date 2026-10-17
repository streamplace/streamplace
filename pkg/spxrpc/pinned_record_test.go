package spxrpc

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func TestCreatePinRejectsInvalidBroadcast(t *testing.T) {
	for _, uri := range []string{
		"not-an-at-uri",
		"at://did:plc:other/place.stream.livestream/1",
		"at://did:plc:streamer/place.stream.chat.message/1",
		"at://did:plc:streamer/place.stream.livestream",
	} {
		t.Run(uri, func(t *testing.T) {
			s := &Server{}
			_, err := s.handlePlaceStreamModerationCreatePin(context.Background(), &placestream.ModerationCreatePin_Input{
				Streamer:   "did:plc:streamer",
				MessageUri: "at://did:plc:viewer/place.stream.chat.message/1",
				Livestream: &uri,
			})
			var httpErr *echo.HTTPError
			require.ErrorAs(t, err, &httpErr)
			require.Equal(t, http.StatusBadRequest, httpErr.Code)
			require.Equal(t, "livestream must reference the streamer's broadcast", httpErr.Message)
		})
	}
}
