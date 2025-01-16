package aqhttp

import "net/http"

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
	}
}
