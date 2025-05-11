package oproxy

type PAR struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
	LoginHint           string `json:"login_hint"`
	ResponseMode        string `json:"response_mode"`
	ResponseType        string `json:"response_type"`
	Scope               string `json:"scope"`
}

type PARResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
}
