package middleware

import (
	"net/http"

	"clinicapp/backend/internal/renderer"
)

// ClientType reads X-Client-Type off the request and stores the resolved
// value on the context for downstream handlers and the renderer.
func ClientType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := renderer.ClientWeb
		if r.Header.Get("X-Client-Type") == "mobile" {
			ct = renderer.ClientMobile
		}
		ctx := renderer.WithClientType(r.Context(), ct)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
