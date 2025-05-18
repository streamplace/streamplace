package errors

import (
	"context"
	"encoding/json"
	"net/http"

	"stream.place/streamplace/pkg/log"
)

type APIError struct {
	Msg    string `json:"message"`
	Status int    `json:"status"`
	Err    error  `json:"-"`
}

func writeHTTPError(w http.ResponseWriter, msg string, status int, err error) APIError {
	w.WriteHeader(status)

	var errorDetail string
	if err != nil {
		errorDetail = err.Error()
	}

	if err != nil {
		log.Log(context.TODO(), msg, "status", status, "error", err)
	} else {
		log.Log(context.TODO(), msg, "status", status)
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg, "error_detail": errorDetail}); err != nil {
		log.Log(context.TODO(), "error writing HTTP error", "http_error_msg", msg, "error", err)
	}
	return APIError{msg, status, err}
}

// HTTP Errors
func WriteHTTPUnauthorized(w http.ResponseWriter, msg string, err error) APIError {
	return writeHTTPError(w, msg, http.StatusUnauthorized, err)
}

func WriteHTTPForbidden(w http.ResponseWriter, msg string, err error) APIError {
	return writeHTTPError(w, msg, http.StatusForbidden, err)
}

func WriteHTTPBadRequest(w http.ResponseWriter, msg string, err error) APIError {
	return writeHTTPError(w, msg, http.StatusBadRequest, err)
}

func WriteHTTPUnsupportedMediaType(w http.ResponseWriter, msg string, err error) APIError {
	return writeHTTPError(w, msg, http.StatusUnsupportedMediaType, err)
}

func WriteHTTPNotFound(w http.ResponseWriter, msg string, err error) APIError {
	return writeHTTPError(w, msg, http.StatusNotFound, err)
}

func WriteHTTPInternalServerError(w http.ResponseWriter, msg string, err error) APIError {
	return writeHTTPError(w, msg, http.StatusInternalServerError, err)
}

func WriteHTTPNotImplemented(w http.ResponseWriter, msg string, err error) APIError {
	return writeHTTPError(w, msg, http.StatusNotImplemented, err)
}

func WriteHTTPTooManyRequests(w http.ResponseWriter, msg string) APIError {
	return writeHTTPError(w, msg, http.StatusTooManyRequests, nil)
}
