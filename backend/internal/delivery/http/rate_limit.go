package httpdelivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

const (
	defaultMCPRateLimitPerMinute = 300
	defaultMCPRateLimitBurst     = 60
	defaultMCPRateLimitBucketTTL = 10 * time.Minute
	defaultMCPRateLimitSweep     = time.Minute
	defaultMCPRateLimitIPGuard   = 10
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
	guardBuckets      map[string]*rateLimitBucket
	bucketTTL         time.Duration
	sweepInterval     time.Duration
	lastSweep         time.Time
	now               func() time.Time
}

type rateLimitBucket struct {
	tokens float64
	last   time.Time
}

type rateLimitStatusContextKey struct{}

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
		guardBuckets:      map[string]*rateLimitBucket{},
		bucketTTL:         defaultMCPRateLimitBucketTTL,
		sweepInterval:     defaultMCPRateLimitSweep,
		now:               time.Now,
	}
}

func (l *rateLimiter) wrap(scope string, next http.HandlerFunc) http.HandlerFunc {
	if l == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		guardKey := ""
		if bearerToken(r) != "" {
			guardKey = scope + ":guard:" + requestNetworkClientKey(r)
			guardAllowed, guardRetryAfter := l.allowNetworkGuard(guardKey)
			if !guardAllowed {
				w.Header().Set("Retry-After", strconv.Itoa(guardRetryAfter))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}
		clientKey := scope + ":" + requestClientKey(r)
		allowed, retryAfter := l.allow(clientKey)
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		status := l.status(scope, clientKey, guardKey)
		ctx := context.WithValue(r.Context(), rateLimitStatusContextKey{}, status)
		next(w, r.WithContext(ctx))
	}
}

func (l *rateLimiter) status(scope string, clientKey string, guardKey string) domainmcp.RateLimitStatus {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	status := domainmcp.RateLimitStatus{
		Enabled:    true,
		Algorithm:  "token_bucket",
		Scope:      scope,
		ObservedAt: now.Unix(),
	}
	client := snapshotRateLimitBucket(l.buckets, clientKey, l.requestsPerMinute, l.burst, now)
	status.Client = &client
	if guardKey != "" {
		guard := snapshotRateLimitBucket(
			l.guardBuckets,
			guardKey,
			l.requestsPerMinute*defaultMCPRateLimitIPGuard,
			l.burst*defaultMCPRateLimitIPGuard,
			now,
		)
		status.NetworkGuard = &guard
	}
	return status
}

func rateLimitStatusFromContext(ctx context.Context) *domainmcp.RateLimitStatus {
	status, ok := ctx.Value(rateLimitStatusContextKey{}).(domainmcp.RateLimitStatus)
	if !ok {
		return nil
	}
	return &status
}

func (l *rateLimiter) allow(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)
	return allowRateLimitBucket(l.buckets, key, l.requestsPerMinute, l.burst, now)
}

func (l *rateLimiter) allowNetworkGuard(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweepLocked(now)
	return allowRateLimitBucket(l.guardBuckets, key, l.requestsPerMinute*defaultMCPRateLimitIPGuard, l.burst*defaultMCPRateLimitIPGuard, now)
}

func (l *rateLimiter) sweepLocked(now time.Time) {
	if !l.lastSweep.IsZero() && now.Sub(l.lastSweep) < l.sweepInterval {
		return
	}
	for _, buckets := range []map[string]*rateLimitBucket{l.buckets, l.guardBuckets} {
		for bucketKey, candidate := range buckets {
			if now.Sub(candidate.last) > l.bucketTTL {
				delete(buckets, bucketKey)
			}
		}
	}
	l.lastSweep = now
}

func allowRateLimitBucket(buckets map[string]*rateLimitBucket, key string, requestsPerMinute int, burst int, now time.Time) (bool, int) {
	bucket, ok := buckets[key]
	if !ok {
		buckets[key] = &rateLimitBucket{tokens: float64(burst - 1), last: now}
		return true, 0
	}

	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		refillPerSecond := float64(requestsPerMinute) / 60.0
		bucket.tokens = math.Min(float64(burst), bucket.tokens+elapsed*refillPerSecond)
		bucket.last = now
	}
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}
	seconds := int(math.Ceil((1 - bucket.tokens) / (float64(requestsPerMinute) / 60.0)))
	if seconds < 1 {
		seconds = 1
	}
	return false, seconds
}

func snapshotRateLimitBucket(buckets map[string]*rateLimitBucket, key string, requestsPerMinute int, burst int, now time.Time) domainmcp.RateLimitBucketStatus {
	tokens := float64(burst)
	if bucket, ok := buckets[key]; ok {
		tokens = bucket.tokens
		if elapsed := now.Sub(bucket.last).Seconds(); elapsed > 0 {
			tokens = math.Min(float64(burst), tokens+elapsed*(float64(requestsPerMinute)/60.0))
		}
	}
	if tokens < 0 {
		tokens = 0
	}
	if tokens > float64(burst) {
		tokens = float64(burst)
	}
	refillPerSecond := float64(requestsPerMinute) / 60.0
	retryAfter := 0
	if tokens < 1 {
		retryAfter = int(math.Ceil((1 - tokens) / refillPerSecond))
		if retryAfter < 1 {
			retryAfter = 1
		}
	}
	resetAfter := int(math.Ceil((float64(burst) - tokens) / refillPerSecond))
	return domainmcp.RateLimitBucketStatus{
		RequestsPerMinute: requestsPerMinute,
		Burst:             burst,
		Remaining:         int(math.Floor(tokens)),
		RefillPerSecond:   refillPerSecond,
		RetryAfterSeconds: retryAfter,
		ResetAt:           now.Add(time.Duration(resetAfter) * time.Second).Unix(),
	}
}

func requestClientKey(r *http.Request) string {
	if token := bearerToken(r); token != "" {
		sum := sha256.Sum256([]byte(token))
		return "token:" + hex.EncodeToString(sum[:16])
	}
	return requestNetworkClientKey(r)
}

func requestNetworkClientKey(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		if comma := strings.IndexByte(forwarded, ','); comma >= 0 {
			forwarded = forwarded[:comma]
		}
		if forwarded = strings.TrimSpace(forwarded); net.ParseIP(forwarded) != nil {
			return "ip:" + forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return "ip:" + host
	}
	if r.RemoteAddr != "" {
		return "remote:" + r.RemoteAddr
	}
	return "unknown"
}
