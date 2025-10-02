package aqhttp

import (
	"net/http"
	"time"
)

var Client http.Client
var UserAgent string = "streamplace/unknown"

type AddHeaderTransport struct {
	T http.RoundTripper
}

func (adt *AddHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", UserAgent)
	return adt.T.RoundTrip(req)
}

func init() {
	Client = http.Client{
		Transport: &AddHeaderTransport{T: &http.Transport{}},
		// do not follow redirects automatically
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		// add reasonable timeout
		Timeout: 30 * time.Second,
	}
}
