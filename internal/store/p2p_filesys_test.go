package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"sqlite-p2p/pkg/p2p"
)

func TestFilesys_PutGetDelete(t *testing.T) {
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

	// 1. PutFile with nil metadata
	ts1 := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	content1 := []byte("hello world")
	err = backend.PutFile(ctx, "docs/hello.txt", nil, content1, ts1)
	if err != nil {
		t.Fatalf("PutFile failed: %v", err)
	}

	// 2. GetFile
	entry, err := backend.GetFile(ctx, "docs/hello.txt")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil FileEntry")
	}
	if entry.Path != "docs/hello.txt" {
		t.Fatalf("expected path 'docs/hello.txt', got '%s'", entry.Path)
	}
	if string(entry.Content) != "hello world" {
		t.Fatalf("expected content 'hello world', got '%s'", string(entry.Content))
	}
	if entry.Metadata.Valid {
		t.Fatalf("expected invalid (null) metadata, got %s", entry.Metadata.String)
	}
	if !entry.Timestamp.Equal(ts1) {
		t.Fatalf("expected timestamp %v, got %v", ts1, entry.Timestamp)
	}

	// 3. PutFile with map metadata
	ts2 := time.Date(2026, 9, 23, 11, 0, 0, 0, time.UTC)
	meta := map[string]interface{}{
		"mime_type": "text/plain",
		"owner":     "alice",
	}
	content2 := []byte("hello updated")
	err = backend.PutFile(ctx, "docs/hello.txt", meta, content2, ts2)
	if err != nil {
		t.Fatalf("PutFile update failed: %v", err)
	}

	entry2, err := backend.GetFile(ctx, "docs/hello.txt")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if !entry2.Metadata.Valid {
		t.Fatal("expected valid metadata")
	}
	var metaParsed map[string]interface{}
	if err := json.Unmarshal([]byte(entry2.Metadata.String), &metaParsed); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if metaParsed["mime_type"] != "text/plain" || metaParsed["owner"] != "alice" {
		t.Fatalf("unexpected metadata content: %v", metaParsed)
	}
	if string(entry2.Content) != "hello updated" {
		t.Fatalf("expected updated content, got %s", string(entry2.Content))
	}

	// 4. GetFile non-existent
	nonExistent, err := backend.GetFile(ctx, "nonexistent.txt")
	if err != nil {
		t.Fatalf("unexpected error for non-existent file: %v", err)
	}
	if nonExistent != nil {
		t.Fatalf("expected nil for non-existent file, got %+v", nonExistent)
	}

	// 5. DeleteFile
	err = backend.DeleteFile(ctx, "docs/hello.txt")
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
	deleted, err := backend.GetFile(ctx, "docs/hello.txt")
	if err != nil {
		t.Fatalf("unexpected error getting deleted file: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected nil for deleted file, got %+v", deleted)
	}

	// Verify changesets were captured
	if len(emittedChanges) < 3 {
		t.Fatalf("expected at least 3 changesets emitted, got %d", len(emittedChanges))
	}
}

func TestFilesys_ListAndGetAll(t *testing.T) {
	ctx := context.Background()
	backend, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	now := time.Now().UTC().Truncate(time.Second)

	_ = backend.PutFile(ctx, "images/pic1.png", `{"mime_type":"image/png"}`, []byte("png1"), now.Add(-3*time.Hour))
	_ = backend.PutFile(ctx, "images/pic2.png", `{"mime_type":"image/png"}`, []byte("png2"), now.Add(-2*time.Hour))
	_ = backend.PutFile(ctx, "logs/2026-09-23.log", `{"mime_type":"text/plain"}`, []byte("log data"), now.Add(-1*time.Hour))

	// Test ListFiles with prefix
	imgFiles, err := backend.ListFiles(ctx, "images/", 10)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(imgFiles) != 2 {
		t.Fatalf("expected 2 image files, got %d", len(imgFiles))
	}
	// Ordered by tmstamp DESC: pic2 then pic1
	if imgFiles[0].Path != "images/pic2.png" {
		t.Fatalf("expected pic2.png first, got %s", imgFiles[0].Path)
	}

	// Test ListFiles with default limit
	allListed, err := backend.ListFiles(ctx, "", 0)
	if err != nil {
		t.Fatalf("ListFiles all failed: %v", err)
	}
	if len(allListed) != 3 {
		t.Fatalf("expected 3 files, got %d", len(allListed))
	}

	// Test GetAllFiles (ordered by tmstamp ASC)
	allFiles, err := backend.GetAllFiles(ctx)
	if err != nil {
		t.Fatalf("GetAllFiles failed: %v", err)
	}
	if len(allFiles) != 3 {
		t.Fatalf("expected 3 files, got %d", len(allFiles))
	}
	if allFiles[0].Path != "images/pic1.png" || allFiles[2].Path != "logs/2026-09-23.log" {
		t.Fatalf("expected ASC order, got %s then %s", allFiles[0].Path, allFiles[2].Path)
	}
}

func TestFilesys_MetadataStringAndBytes(t *testing.T) {
	ctx := context.Background()
	backend, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	ts := time.Now().UTC()
	err = backend.PutFile(ctx, "test/bytes.txt", []byte(`{"key":"val"}`), []byte("content"), ts)
	if err != nil {
		t.Fatalf("PutFile with bytes failed: %v", err)
	}

	entry, err := backend.GetFile(ctx, "test/bytes.txt")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if !entry.Metadata.Valid || entry.Metadata.String != `{"key":"val"}` {
		t.Fatalf("unexpected metadata: %v", entry.Metadata)
	}
}
