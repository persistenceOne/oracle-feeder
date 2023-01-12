package httputil

import (
	"encoding/json"
	"net/http"
)

// Common HTTP methods and header values
const (
	MethodGET = "GET"
)

// ErrResponse defines an HTTP error response.
type ErrResponse struct {
	Error string `json:"error"`
}

// RespondWithJSON provides an auxiliary function to return an HTTP response
// with JSON content and an HTTP status code.
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}
