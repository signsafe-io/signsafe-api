package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	ContextKeyUserID   contextKey = "userID"
	ContextKeyUserRole contextKey = "userRole"
	ContextKeyOrgID    contextKey = "orgID"
)

// Claims holds the JWT payload.
type Claims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	OrgID  string `json:"orgId"`
	jwt.RegisteredClaims
}

// Authenticate validates the Bearer JWT token in the Authorization header.
func Authenticate(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w, "invalid Authorization header format")
				return
			}

			tokenStr := parts[1]
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				writeUnauthorized(w, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextKeyUserRole, claims.Role)
			ctx = context.WithValue(ctx, ContextKeyOrgID, claims.OrgID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyUserID).(string)
	return v
}

// UserRoleFromContext extracts the user role from the request context.
func UserRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyUserRole).(string)
	return v
}

// OrgIDFromContext extracts the organization ID embedded in the JWT from the request context.
// Returns an empty string for tokens issued before this field was added.
func OrgIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ContextKeyOrgID).(string)
	return v
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
