package store

import (
	"context"
	"testing"
	"time"

	"sqlite-p2p/pkg/p2p"
)

func TestBlacklist_Operations(t *testing.T) {
	ctx := context.Background()
	backend, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	// Track changesets
	var emittedChanges []*p2p.Changeset
	backend.Tracker().Subscribe(func(cs *p2p.Changeset, raw []byte) {
		emittedChanges = append(emittedChanges, cs)
	})

	phone1 := "+1234567890"
	phone2 := "+9876543210"

	// 1. Initial check - neither should be blacklisted
	isBl, err := backend.IsBlacklisted(ctx, phone1)
	if err != nil {
		t.Fatalf("IsBlacklisted failed: %v", err)
	}
	if isBl {
		t.Fatalf("expected false for initial check")
	}

	// 2. Add to blacklist
	if err := backend.AddBlacklist(ctx, phone1, "Spam calls"); err != nil {
		t.Fatalf("AddBlacklist phone1 failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := backend.AddBlacklist(ctx, phone2, "Harassment"); err != nil {
		t.Fatalf("AddBlacklist phone2 failed: %v", err)
	}

	// 3. Check IsBlacklisted
	isBl1, err := backend.IsBlacklisted(ctx, phone1)
	if err != nil || !isBl1 {
		t.Fatalf("expected true for phone1, got %v (err: %v)", isBl1, err)
	}
	isBl2, err := backend.IsBlacklisted(ctx, phone2)
	if err != nil || !isBl2 {
		t.Fatalf("expected true for phone2, got %v (err: %v)", isBl2, err)
	}

	// 4. ListBlacklist (ordered by created_at DESC)
	list, err := backend.ListBlacklist(ctx)
	if err != nil {
		t.Fatalf("ListBlacklist failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 blacklisted numbers, got %d", len(list))
	}
	if list[0].Phone != phone2 || list[1].Phone != phone1 {
		t.Fatalf("expected DESC order (phone2 then phone1), got %s then %s", list[0].Phone, list[1].Phone)
	}
	if list[0].Reason != "Harassment" || list[1].Reason != "Spam calls" {
		t.Fatalf("unexpected reasons: %s, %s", list[0].Reason, list[1].Reason)
	}

	// 5. Remove from blacklist
	if err := backend.RemoveBlacklist(ctx, phone1); err != nil {
		t.Fatalf("RemoveBlacklist phone1 failed: %v", err)
	}

	isBl1After, err := backend.IsBlacklisted(ctx, phone1)
	if err != nil || isBl1After {
		t.Fatalf("expected false for phone1 after removal, got %v (err: %v)", isBl1After, err)
	}

	listAfter, err := backend.ListBlacklist(ctx)
	if err != nil {
		t.Fatalf("ListBlacklist after removal failed: %v", err)
	}
	if len(listAfter) != 1 || listAfter[0].Phone != phone2 {
		t.Fatalf("expected only phone2 remaining, got %v", listAfter)
	}

	// 6. Verify changesets were captured (2 puts + 1 delete)
	if len(emittedChanges) < 3 {
		t.Fatalf("expected at least 3 changesets, got %d", len(emittedChanges))
	}
}
