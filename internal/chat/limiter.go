// Package chat manages real-time WebSocket communication.
package chat

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter tracks and enforces rate limits per client IP address.
type IPRateLimiter struct {
	mu          sync.Mutex
	limiters    map[string]*ipLimiterEntry
	limit       rate.Limit
	burst       int
	cleanupFreq time.Duration
	lastCleanup time.Time
}

type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a new per-IP rate limiter with the given rate and burst.
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters:    make(map[string]*ipLimiterEntry),
		limit:       r,
		burst:       burst,
		cleanupFreq: 5 * time.Minute,
		lastCleanup: time.Now(),
	}
}

// Allow reports whether an event from the given IP may happen now.
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Opportunistic cleanup of stale IP entries
	if time.Since(l.lastCleanup) > l.cleanupFreq {
		now := time.Now()
		for k, v := range l.limiters {
			if now.Sub(v.lastSeen) > 10*time.Minute {
				delete(l.limiters, k)
			}
		}
		l.lastCleanup = now
	}

	entry, exists := l.limiters[ip]
	if !exists {
		entry = &ipLimiterEntry{
			limiter:  rate.NewLimiter(l.limit, l.burst),
			lastSeen: time.Now(),
		}
		l.limiters[ip] = entry
	} else {
		entry.lastSeen = time.Now()
	}

	return entry.limiter.Allow()
}

// ClientIP extracts the client IP address from an HTTP request.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ip := strings.TrimSpace(xrip)
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
