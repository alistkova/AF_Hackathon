package identity

import (
	"sync"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
)

const (
	defaultLoginMaxAttempts = 5
	defaultLoginWindow      = 15 * time.Minute
)

type loginAttempt struct {
	count        int
	first        time.Time
	blockedUntil time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	max      int
	window   time.Duration
	now      func() time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		attempts: map[string]loginAttempt{},
		max:      config.Int("IDENTITY_LOGIN_MAX_ATTEMPTS", defaultLoginMaxAttempts),
		window:   config.Duration("IDENTITY_LOGIN_WINDOW", defaultLoginWindow),
		now:      time.Now,
	}
}

func (l *loginLimiter) allow(key string) (bool, time.Duration) {
	if l.max <= 0 || l.window <= 0 {
		return true, 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	attempt, ok := l.attempts[key]
	if !ok {
		return true, 0
	}
	if !attempt.blockedUntil.IsZero() && now.Before(attempt.blockedUntil) {
		return false, attempt.blockedUntil.Sub(now)
	}
	if now.Sub(attempt.first) >= l.window {
		delete(l.attempts, key)
		return true, 0
	}
	if attempt.count >= l.max {
		attempt.blockedUntil = attempt.first.Add(l.window)
		l.attempts[key] = attempt
		return false, attempt.blockedUntil.Sub(now)
	}
	return true, 0
}

func (l *loginLimiter) fail(key string) {
	if l.max <= 0 || l.window <= 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	attempt, ok := l.attempts[key]
	if !ok || now.Sub(attempt.first) >= l.window {
		attempt = loginAttempt{first: now}
	}
	attempt.count++
	if attempt.count >= l.max {
		attempt.blockedUntil = attempt.first.Add(l.window)
	}
	l.attempts[key] = attempt

	if len(l.attempts) > 10000 {
		for candidate, entry := range l.attempts {
			if now.Sub(entry.first) >= l.window {
				delete(l.attempts, candidate)
			}
		}
	}
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}
