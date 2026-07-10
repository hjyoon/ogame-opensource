package httpdelivery

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultMCPRateLimitPerMinute = 300
	defaultMCPRateLimitBurst     = 60
)

type RateLimitConfig struct {
	Disabled          bool
	RequestsPerMinute int
	Burst             int
}

type rateLimiter struct {
	mu                sync.Mutex
	requestsPerMinute int
	burst             int
	buckets           map[string]*rateLimitBucket
	now               func() time.Time
}

type rateLimitBucket struct {
	tokens float64
	last   time.Time
}

func newMCPRateLimiter(config RateLimitConfig) *rateLimiter {
	if config.Disabled {
		return nil
	}
	requestsPerMinute := config.RequestsPerMinute
	if requestsPerMinute <= 0 {
		requestsPerMinute = defaultMCPRateLimitPerMinute
	}
	burst := config.Burst
	if burst <= 0 {
		burst = defaultMCPRateLimitBurst
	}
	if burst > requestsPerMinute {
		burst = requestsPerMinute
	}
	return &rateLimiter{
		requestsPerMinute: requestsPerMinute,
		burst:             burst,
		buckets:           map[string]*rateLimitBucket{},
		now:               time.Now,
	}
}

func (l *rateLimiter) wrap(scope string, next http.HandlerFunc) http.HandlerFunc {
	if l == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := l.allow(scope + ":" + requestClientKey(r))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func (l *rateLimiter) allow(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	bucket, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &rateLimitBucket{tokens: float64(l.burst - 1), last: now}
		return true, 0
	}

	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		refillPerSecond := float64(l.requestsPerMinute) / 60.0
		bucket.tokens = math.Min(float64(l.burst), bucket.tokens+elapsed*refillPerSecond)
		bucket.last = now
	}
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}
	seconds := int(math.Ceil((1 - bucket.tokens) / (float64(l.requestsPerMinute) / 60.0)))
	if seconds < 1 {
		seconds = 1
	}
	return false, seconds
}

func requestClientKey(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		if comma := strings.IndexByte(forwarded, ','); comma >= 0 {
			forwarded = forwarded[:comma]
		}
		if forwarded = strings.TrimSpace(forwarded); forwarded != "" {
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}
