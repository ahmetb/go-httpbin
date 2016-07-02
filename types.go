package httpbin

type ipResponse struct {
	Origin string `json:"origin"`
}

type errorResponse struct {
	Error errObj `json:"error"`
}

type errObj struct {
	Message string `json:"message"`
}

type userAgentResponse struct {
	UA string `json:"user-agent"`
}
