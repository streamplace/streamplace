package spxrpc

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	placestreamtypes "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

func (s *Server) handlePlaceStreamServerUpsertStorage(ctx context.Context, input *placestreamtypes.ServerUpsertStorage_Input) (*placestreamtypes.ServerUpsertStorage_Output, error) {
	// Get authenticated user
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// Get existing storage if any
	existing, _ := s.statefulDB.GetStorage(session.DID)

	var url string
	if input.Url != nil {
		url = *input.Url
		if existing != nil && url != "" {
			maskedExisting := existing.ToLexicon().Url
			if url == maskedExisting {
				url = existing.URL
			}
		}

		// if url contains masked password, return an error
		if strings.Contains(url, ":***@") {
			return nil, &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "MaskedCredentialsModified", Message: "Cannot modify URL while keeping masked credentials. Provide full credentials or omit URL to keep existing configuration."},
			}
		}

		if url != "" && !isValidS3URL(url) {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid S3 URL format. Expected: s3+https://ACCESS_KEY:SECRET_KEY@endpoint/bucket")
		}
	} else if existing != nil {
		url = existing.URL
	}

	storage := statedb.StorageFromLexiconInput(*input, session.DID)
	storage.URL = url
	if input.IsActive == nil && existing != nil {
		storage.IsActive = existing.IsActive
	}

	err := s.statefulDB.UpsertStorage(storage)
	if err != nil {
		log.Error(ctx, "failed to upsert storage", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to save storage configuration")
	}

	savedStorage, err := s.statefulDB.GetStorage(session.DID)
	if err != nil {
		log.Error(ctx, "failed to get storage after upsert", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to retrieve storage configuration")
	}

	return &placestreamtypes.ServerUpsertStorage_Output{
		Storage: savedStorage.ToLexicon(),
	}, nil
}

func (s *Server) handlePlaceStreamServerGetStorage(ctx context.Context) (*placestreamtypes.ServerGetStorage_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	storage, err := s.statefulDB.GetStorage(session.DID)
	if err != nil {
		return &placestreamtypes.ServerGetStorage_Output{
			Storage: nil,
		}, nil
	}

	storageLex := storage.ToLexicon()
	return &placestreamtypes.ServerGetStorage_Output{
		Storage: &storageLex,
	}, nil
}

func (s *Server) handlePlaceStreamServerDeleteStorage(ctx context.Context) (*placestreamtypes.ServerDeleteStorage_Output, error) {
	// Get authenticated user
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// Delete storage
	err := s.statefulDB.DeleteStorage(session.DID)
	if err != nil {
		log.Error(ctx, "failed to delete storage", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete storage configuration")
	}

	return &placestreamtypes.ServerDeleteStorage_Output{
		Success: true,
	}, nil
}

// isValidS3URL validates the S3 URL format
func isValidS3URL(url string) bool {
	// Format: s3+https://ACCESS_KEY:SECRET_KEY@endpoint/bucket
	re := regexp.MustCompile(`^s3\+https?://[^:]+:[^@]+@[^/]+/.+$`)
	return re.MatchString(url)
}
