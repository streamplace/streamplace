package spxrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
)

func (s *Server) handleComAtprotoModerationCreateReport(ctx context.Context, body *comatprototypes.ModerationCreateReport_Input) (*comatprototypes.ModerationCreateReport_Output, error) {
	c, ok := ctx.Value(echoContextKey).(echo.Context)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "echo context not found")
	}

	dPoP := c.Request().Header.Get("DPoP")
	if dPoP == "" {
		return s.handleComAtprotoModerationCreateReportToOzone(ctx, c, body)
	} else {
		return s.handleComAtprotoModerationCreateReportToPDS(ctx, c, body)
	}
}

func (s *Server) handleComAtprotoModerationCreateReportToOzone(ctx context.Context, c echo.Context, body *comatprototypes.ModerationCreateReport_Input) (*comatprototypes.ModerationCreateReport_Output, error) {
	// Serialize the input body to JSON
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to marshal request body: %v", err))
	}

	// Create the request URL
	url := fmt.Sprintf("%s/xrpc/com.atproto.moderation.createReport", s.cli.OzoneURL)

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create request: %v", err))
	}

	authorization := c.Request().Header.Get("Authorization")
	if authorization == "" {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Authorization header not found")
	}

	log.Log(ctx, "handleComAtprotoModerationCreateReportToOzone", "authorization", authorization)

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorization)

	// Send the request
	resp, err := aqhttp.Client.Do(req)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to send request: %v", err))
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to read response body: %v", err))
	}

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return nil, echo.NewHTTPError(resp.StatusCode, fmt.Sprintf("upstream error: %s", string(respBody)))
	}

	// Deserialize the response
	var output comatprototypes.ModerationCreateReport_Output
	if err := json.Unmarshal(respBody, &output); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to unmarshal response: %v", err))
	}

	return &output, nil
}

func (s *Server) handleComAtprotoModerationCreateReportToPDS(ctx context.Context, c echo.Context, body *comatprototypes.ModerationCreateReport_Input) (*comatprototypes.ModerationCreateReport_Output, error) {
	log.Log(ctx, "handleComAtprotoModerationCreateReport", "body", body)
	if s.cli.OzoneURL == "" {
		return nil, echo.NewHTTPError(http.StatusNotImplemented, "Ozone URL is not set")
	}

	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	client.SetHeaders(map[string]string{
		"Atproto-Proxy": fmt.Sprintf("%s#atproto_labeler", s.cli.MyDID()),
	})

	var output comatprototypes.ModerationCreateReport_Output
	err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.moderation.createReport", nil, body, &output)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return &output, nil
}
