package renderer

import (
	"context"
	"encoding/json"
	"net/http"
)

type ClientType string

const (
	ClientWeb    ClientType = "web"
	ClientMobile ClientType = "mobile"
)

type ctxKey struct{}

// WithClientType stores the resolved client type on the request context.
func WithClientType(ctx context.Context, ct ClientType) context.Context {
	return context.WithValue(ctx, ctxKey{}, ct)
}

// ClientTypeFrom reads the client type from context, defaulting to ClientWeb
// when absent (matches the X-Client-Type contract: web is the default).
func ClientTypeFrom(ctx context.Context) ClientType {
	ct, ok := ctx.Value(ctxKey{}).(ClientType)
	if !ok {
		return ClientWeb
	}
	return ct
}

// Response is what a handler builds; Render picks the right representation
// based on the caller's client type. Mobile always gets JSON. Web gets a
// redirect if RedirectURL is set, otherwise the HTML fragment.
type Response struct {
	Status      int
	JSON        any
	HTML        string
	RedirectURL string
}

func Render(w http.ResponseWriter, r *http.Request, resp Response) {
	if ClientTypeFrom(r.Context()) == ClientMobile {
		writeJSON(w, resp.Status, resp.JSON)
		return
	}

	if resp.RedirectURL != "" {
		http.Redirect(w, r, resp.RedirectURL, http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(resp.Status)
	w.Write([]byte(resp.HTML))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
