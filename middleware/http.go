package middleware

import (
	"net"
	"net/http"

	"github.com/serroba/rate/registry"
)

// KeyFunc extracts a rate limit key from an HTTP request.
// Common implementations include extracting client IP, API key, or user ID.
type KeyFunc func(r *http.Request) registry.Identifier

// IPKeyFunc extracts the client IP address from the request.
// It checks X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr.
func IPKeyFunc(r *http.Request) registry.Identifier {
	// Check X-Forwarded-For first (may contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		for i := range len(xff) {
			if xff[i] == ',' {
				return registry.Identifier(xff[:i])
			}
		}

		return registry.Identifier(xff)
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return registry.Identifier(xri)
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return registry.Identifier(r.RemoteAddr)
	}

	return registry.Identifier(host)
}

// HeaderKeyFunc returns a KeyFunc that extracts the rate limit key from a header.
// Useful for API key or token-based rate limiting.
func HeaderKeyFunc(header string) KeyFunc {
	return func(r *http.Request) registry.Identifier {
		return registry.Identifier(r.Header.Get(header))
	}
}

// RateLimiter returns HTTP middleware that rate limits requests.
// It uses the provided registry to track rate limits per key extracted by keyFunc.
// Requests that exceed the rate limit receive a 429 Too Many Requests response.
func RateLimiter(reg *registry.Registry, keyFunc KeyFunc) func(http.Handler) http.Handler {
	if keyFunc == nil {
		keyFunc = IPKeyFunc
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)

			if !reg.Allow(key) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
