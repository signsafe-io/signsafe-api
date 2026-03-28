package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/signsafe-io/signsafe-api/internal/cache"
)

// RateLimiter returns a middleware that limits requests per IP per endpoint.
// limit is the maximum number of requests allowed per window.
// window is the duration of the sliding window.
// On Redis failure the middleware fails open (allows the request).
func RateLimiter(cacheClient *cache.Client, limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			// Normalise path to avoid cache key collisions with path parameters.
			path := r.URL.Path
			key := fmt.Sprintf("ratelimit:%s:%s", path, ip)

			count, err := cacheClient.Incr(r.Context(), key, window)
			if err != nil {
				// Fail open — do not block requests on Redis errors.
				next.ServeHTTP(w, r)
				return
			}

			if count > limit {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"too many requests, please try again later"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the real client IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the first IP in the chain (original client).
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
