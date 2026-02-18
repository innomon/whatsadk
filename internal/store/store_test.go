package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_gateway.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIsBlacklisted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	ok, err := s.IsBlacklisted(ctx, "910987654321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not blacklisted")
	}

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	ok, err = s.IsBlacklisted(ctx, "910987654321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected blacklisted")
	}
}

func TestRemoveBlacklist(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	if err := s.RemoveBlacklist(ctx, "910987654321"); err != nil {
		t.Fatalf("failed to remove: %v", err)
	}

	ok, err := s.IsBlacklisted(ctx, "910987654321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not blacklisted after removal")
	}
}

func TestListBlacklist(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := s.AddBlacklist(ctx, "910111111111", "abuse"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	list, err := s.ListBlacklist(ctx)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
}

func TestAddBlacklist_Duplicate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AddBlacklist(ctx, "910987654321", "spam"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := s.AddBlacklist(ctx, "910987654321", "spam again"); err != nil {
		t.Fatalf("duplicate add should not error: %v", err)
	}

	list, err := s.ListBlacklist(ctx)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 entry after duplicate add, got %d", len(list))
	}
}
