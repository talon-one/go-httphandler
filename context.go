package httphandler

import "net/http"

type contextKey int

const (
	uuidKey contextKey = iota
)

// GetRequestUUID returns the request uuid for the specified request.
func GetRequestUUID(r *http.Request) string {
	if rv := r.Context().Value(uuidKey); rv != nil {
		return rv.(string)
	}
	// should not be possible
	return ""
}
