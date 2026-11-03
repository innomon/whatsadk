package verification

import (
	"sync"
	"time"
)

// RateLimiter tracks verification attempts per sender within a sliding time window.
type RateLimiter struct {
	mu          sync.Mutex
	maxAttempts int
	window      time.Duration
	records     map[string][]time.Time
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if window <= 0 {
		window = 1 * time.Minute
	}
	return &RateLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		records:     make(map[string][]time.Time),
	}
}

// Allow reports whether a verification request from sender is permitted right now.
func (rl *RateLimiter) Allow(sender string) bool {
	return rl.AllowAt(sender, time.Now())
}

// AllowAt reports whether a verification request from sender is permitted at the specified time.
func (rl *RateLimiter) AllowAt(sender string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	key := normalizePhone(sender)
	cutoff := now.Add(-rl.window)

	existing := rl.records[key]
	var valid []time.Time
	for _, t := range existing {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.maxAttempts {
		rl.records[key] = valid
		return false
	}

	valid = append(valid, now)
	rl.records[key] = valid
	return true
}

// Reset clears the rate limit history for a specific sender.
func (rl *RateLimiter) Reset(sender string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.records, normalizePhone(sender))
}

// Cleanup removes all expired records based on the given cutoff time.
func (rl *RateLimiter) Cleanup(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := now.Add(-rl.window)
	for key, timestamps := range rl.records {
		var valid []time.Time
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.records, key)
		} else {
			rl.records[key] = valid
		}
	}
}
