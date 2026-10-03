package spxrpc

import (
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPushCaptionsBoundsChunkedRequestBody(t *testing.T) {
	e := echo.New()
	e.Use(captionPushBodyLimitMiddleware())
	e.POST("/xrpc/place.stream.caption.pushCaptions", func(c echo.Context) error {
		var value map[string]any
		if err := c.Bind(&value); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
	body := `{"language":"en","padding":"` + strings.Repeat("x", 2<<20) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/xrpc/place.stream.caption.pushCaptions", io.NopCloser(strings.NewReader(body)))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.ContentLength = -1
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
}
