package chat_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/chat"
	"golang.org/x/time/rate"
)

func TestIPRateLimiter(t *testing.T) {
	// Rate of 1 per second, burst 2
	limiter := chat.NewIPRateLimiter(rate.Limit(1), 2)

	ipA := "192.168.1.1"
	ipB := "192.168.1.2"

	// ipA: first 2 should succeed (burst = 2)
	if !limiter.Allow(ipA) {
		t.Errorf("expected ipA request 1 to be allowed")
	}
	if !limiter.Allow(ipA) {
		t.Errorf("expected ipA request 2 to be allowed")
	}
	// ipA: 3rd should fail
	if limiter.Allow(ipA) {
		t.Errorf("expected ipA request 3 to be rejected")
	}

	// ipB: should have independent limit and be allowed
	if !limiter.Allow(ipB) {
		t.Errorf("expected ipB request 1 to be allowed")
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		wantIP     string
	}{
		{
			name:       "remote addr with port",
			headers:    nil,
			remoteAddr: "203.0.113.195:4321",
			wantIP:     "203.0.113.195",
		},
		{
			name: "x-forwarded-for single",
			headers: map[string]string{
				"X-Forwarded-For": "198.51.100.1",
			},
			remoteAddr: "10.0.0.1:1234",
			wantIP:     "198.51.100.1",
		},
		{
			name: "x-forwarded-for multi with spaces",
			headers: map[string]string{
				"X-Forwarded-For": "198.51.100.2, 10.0.0.1, 10.0.0.2",
			},
			remoteAddr: "10.0.0.1:1234",
			wantIP:     "198.51.100.2",
		},
		{
			name: "x-real-ip header",
			headers: map[string]string{
				"X-Real-IP": "198.51.100.3",
			},
			remoteAddr: "10.0.0.1:1234",
			wantIP:     "198.51.100.3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.RemoteAddr = tc.remoteAddr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			got := chat.ClientIP(req)
			if got != tc.wantIP {
				t.Errorf("ClientIP: got %q, want %q", got, tc.wantIP)
			}
		})
	}
}
