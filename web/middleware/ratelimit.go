package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
)

type rateEntry struct {
	count    int
	lastSeen time.Time
}

// RateLimitMiddleware returns a Gin middleware that limits requests per IP.
// maxRequests is the maximum number of requests allowed within the window.
func RateLimitMiddleware(maxRequests int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]*rateEntry)

	// Periodically evict stale entries to prevent unbounded memory growth
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			cutoff := time.Now().Add(-window * 2)
			for ip, e := range entries {
				if e.lastSeen.Before(cutoff) {
					delete(entries, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		// Use RemoteAddr directly to prevent IP spoofing via headers
		ip := c.Request.RemoteAddr

		mu.Lock()
		now := time.Now()
		e, exists := entries[ip]
		if !exists || now.Sub(e.lastSeen) > window {
			entries[ip] = &rateEntry{count: 1, lastSeen: now}
			mu.Unlock()
			c.Next()
			return
		}
		e.lastSeen = now
		e.count++
		if e.count > maxRequests {
			mu.Unlock()
			c.JSON(http.StatusTooManyRequests, entity.Msg{
				Success: false,
				Msg:     "Too many requests",
			})
			c.Abort()
			return
		}
		mu.Unlock()
		c.Next()
	}
}
