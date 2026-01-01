package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/serroba/rate/bucket"
	"github.com/serroba/rate/middleware"
	"github.com/serroba/rate/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRemoteAddr = "10.0.0.1:12345"

func TestIPKeyFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       registry.Identifier
	}{
		{
			name:       "uses X-Forwarded-For first",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1"},
			want:       "10.0.0.1",
		},
		{
			name:       "uses first IP from X-Forwarded-For chain",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1, 10.0.0.2, 10.0.0.3"},
			want:       "10.0.0.1",
		},
		{
			name:       "uses X-Real-IP when no X-Forwarded-For",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Real-IP": "10.0.0.5"},
			want:       "10.0.0.5",
		},
		{
			name:       "prefers X-Forwarded-For over X-Real-IP",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "10.0.0.1",
				"X-Real-IP":       "10.0.0.5",
			},
			want: "10.0.0.1",
		},
		{
			name:       "falls back to RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			headers:    nil,
			want:       "192.168.1.1",
		},
		{
			name:       "handles RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			headers:    nil,
			want:       "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := middleware.IPKeyFunc(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHeaderKeyFunc(t *testing.T) {
	t.Parallel()

	keyFunc := middleware.HeaderKeyFunc("X-Api-Key")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "secret-key-123")

	got := keyFunc(req)
	assert.Equal(t, registry.Identifier("secret-key-123"), got)
}

func TestHeaderKeyFunc_Missing(t *testing.T) {
	t.Parallel()

	keyFunc := middleware.HeaderKeyFunc("X-Api-Key")

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	got := keyFunc(req)
	assert.Equal(t, registry.Identifier(""), got)
}

func TestRateLimiter_Allows(t *testing.T) {
	t.Parallel()

	reg, err := registry.NewRegistry(func() registry.Limiter {
		return bucket.NewTokenLimiter(10, 0)
	})
	require.NoError(t, err)

	handler := middleware.RateLimiter(reg, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testRemoteAddr

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimiter_Blocks(t *testing.T) {
	t.Parallel()

	reg, err := registry.NewRegistry(func() registry.Limiter {
		return bucket.NewTokenLimiter(1, 0)
	})
	require.NoError(t, err)

	handler := middleware.RateLimiter(reg, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = testRemoteAddr

	// First request should pass
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request should be rate limited
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "1", rec.Header().Get("Retry-After"))
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	t.Parallel()

	reg, err := registry.NewRegistry(func() registry.Limiter {
		return bucket.NewTokenLimiter(1, 0)
	})
	require.NoError(t, err)

	handler := middleware.RateLimiter(reg, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First IP exhausts its limit
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = testRemoteAddr

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req1)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req1)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// Second IP should still be allowed
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.2:12345"

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req2)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimiter_CustomKeyFunc(t *testing.T) {
	t.Parallel()

	reg, err := registry.NewRegistry(func() registry.Limiter {
		return bucket.NewTokenLimiter(1, 0)
	})
	require.NoError(t, err)

	keyFunc := middleware.HeaderKeyFunc("X-Api-Key")
	handler := middleware.RateLimiter(reg, keyFunc)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Same IP but different API keys
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = testRemoteAddr
	req1.Header.Set("X-Api-Key", "key-1")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = testRemoteAddr
	req2.Header.Set("X-Api-Key", "key-2")

	// Both should be allowed (different keys)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req1)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req2)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request with key-1 should be blocked
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req1)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimiter_Concurrent(t *testing.T) {
	t.Parallel()

	reg, err := registry.NewRegistry(func() registry.Limiter {
		return bucket.NewTokenLimiter(100, 0)
	})
	require.NoError(t, err)

	var allowed atomic.Int64

	handler := middleware.RateLimiter(reg, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		allowed.Add(1)
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup

	for range 200 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = testRemoteAddr

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(100), allowed.Load())
}
