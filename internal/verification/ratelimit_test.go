package verification

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Minute)
	phone := "+1 (555) 123-4567"
	now := time.Now()

	// First 3 attempts should succeed
	for i := 1; i <= 3; i++ {
		if !rl.AllowAt(phone, now.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("attempt %d should be allowed", i)
		}
	}

	// 4th attempt within the window should fail
	if rl.AllowAt(phone, now.Add(10*time.Second)) {
		t.Fatal("attempt 4 should be rejected by rate limiter")
	}

	// Another sender should not be blocked
	otherPhone := "919876543210"
	if !rl.AllowAt(otherPhone, now.Add(11*time.Second)) {
		t.Fatal("other phone should be allowed")
	}

	// After the window expires, attempts should succeed again
	afterWindow := now.Add(1*time.Minute + 15*time.Second)
	if !rl.AllowAt(phone, afterWindow) {
		t.Fatal("attempt after window expiry should be allowed")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Minute)
	phone := "1234567890"
	now := time.Now()

	rl.AllowAt(phone, now)
	rl.AllowAt(phone, now.Add(1*time.Second))

	if rl.AllowAt(phone, now.Add(2*time.Second)) {
		t.Fatal("expected to be rate limited")
	}

	rl.Reset(phone)

	if !rl.AllowAt(phone, now.Add(3*time.Second)) {
		t.Fatal("expected to be allowed after reset")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(2, 30*time.Second)
	phone1 := "1111111111"
	phone2 := "2222222222"
	now := time.Now()

	rl.AllowAt(phone1, now)
	rl.AllowAt(phone2, now.Add(40*time.Second))

	// Cleanup at now + 35s should expire phone1, retain phone2
	rl.Cleanup(now.Add(35 * time.Second))

	rl.mu.Lock()
	_, p1Exists := rl.records[phone1]
	_, p2Exists := rl.records[phone2]
	rl.mu.Unlock()

	if p1Exists {
		t.Errorf("expected phone1 to be cleaned up")
	}
	if !p2Exists {
		t.Errorf("expected phone2 to be retained")
	}
}
