package middleware

import (
	"context"
	"net/http"
	"strings"

	"clinicapp/backend/internal/auth"
)

type claimsCtxKey struct{}

// ClaimsFrom returns the authenticated request's JWT claims, if the Auth
// middleware ran and the token was valid.
func ClaimsFrom(ctx context.Context) (*auth.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsCtxKey{}).(*auth.AccessClaims)
	return claims, ok
}

// Auth validates the access token (Authorization: Bearer header for mobile,
// "access_token" httpOnly cookie for web) and attaches its claims to the
// request context. Unauthenticated or expired tokens get a 401.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				if cookie, err := r.Cookie("access_token"); err == nil {
					token = cookie.Value
				}
			}
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := auth.ParseAccessToken(jwtSecret, token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), claimsCtxKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
