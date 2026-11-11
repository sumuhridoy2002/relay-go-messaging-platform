package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "user_id"

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		parts := strings.SplitN(header, " ", 2)

		if len(parts) != 2 ||
			!strings.EqualFold(parts[0], "Bearer") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Missing or invalid authorization token",
			})
			return
		}

		claims := &Claims{}

		token, err := jwt.ParseWithClaims(
			parts[1],
			claims,
			func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrSignatureInvalid
				}
				return h.JWTSecret, nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
		)

		if err != nil || !token.Valid || claims.UserID == "" ||
			claims.Subject != claims.UserID {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Invalid or expired access token",
			})
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
