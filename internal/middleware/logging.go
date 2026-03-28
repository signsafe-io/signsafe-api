package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SecurityHeaders adds baseline security headers to every response.
// These are defence-in-depth headers relevant even for a JSON-only API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME-type sniffing.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Deny embedding in frames (clickjacking).
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// Logger logs method, path, status code, duration, and request ID for each request.
// It expects chi's RequestID middleware to run before it so that GetReqID returns a value.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := chimiddleware.GetReqID(r.Context())

		// Echo request ID back to the caller for client-side correlation.
		if reqID != "" {
			w.Header().Set("X-Request-ID", reqID)
		}

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"request_id", reqID,
		)
	})
}
