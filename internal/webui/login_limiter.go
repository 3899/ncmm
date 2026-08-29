package webui

import (
	"sync"
	"time"
)

type loginBucket struct {
	tokens float64
	last   time.Time
}

type loginRateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	clients map[string]loginBucket
	global  loginBucket
}

func newLoginRateLimiter() *loginRateLimiter {
	now := time.Now()
	return &loginRateLimiter{
		now: time.Now, clients: make(map[string]loginBucket),
		global: loginBucket{tokens: 20, last: now},
	}
}

func (l *loginRateLimiter) allow(client string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if client == "" {
		client = "unknown"
	}
	if _, exists := l.clients[client]; !exists && len(l.clients) >= 4096 {
		client = "overflow"
	}
	global := refillBucket(l.global, now, 20, 1.0/15.0)
	local, ok := l.clients[client]
	if !ok {
		local = loginBucket{tokens: 5, last: now}
	} else {
		local = refillBucket(local, now, 5, 1.0/60.0)
	}
	retry := time.Duration(0)
	if global.tokens < 1 {
		retry = bucketRetry(global, 1.0/15.0)
	}
	if local.tokens < 1 {
		if localRetry := bucketRetry(local, 1.0/60.0); localRetry > retry {
			retry = localRetry
		}
	}
	if retry > 0 {
		l.global = global
		l.clients[client] = local
		return false, retry
	}
	global.tokens--
	local.tokens--
	l.global = global
	l.clients[client] = local
	return true, 0
}

func refillBucket(bucket loginBucket, now time.Time, capacity, refillPerSecond float64) loginBucket {
	if bucket.last.IsZero() {
		return loginBucket{tokens: capacity, last: now}
	}
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens = min(capacity, bucket.tokens+elapsed*refillPerSecond)
		bucket.last = now
	}
	return bucket
}

func bucketRetry(bucket loginBucket, refillPerSecond float64) time.Duration {
	seconds := (1 - bucket.tokens) / refillPerSecond
	return max(time.Second, time.Duration(seconds*float64(time.Second)))
}
