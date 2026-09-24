package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"sqlite-p2p/pkg/p2p"
)

func TestCommands_Lifecycle(t *testing.T) {
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

	// 1. Enqueue Command 1
	id1, err := backend.EnqueueCommand(ctx, "send_message", map[string]string{"to": "123", "text": "hello"})
	if err != nil {
		t.Fatalf("EnqueueCommand 1 failed: %v", err)
	}
	if id1 <= 0 {
		t.Fatalf("expected positive ID, got %d", id1)
	}

	// 2. Enqueue Command 2
	id2, err := backend.EnqueueCommand(ctx, "send_image", `{"to":"456","url":"http://example.com/pic.png"}`)
	if err != nil {
		t.Fatalf("EnqueueCommand 2 failed: %v", err)
	}
	if id2 != id1+1 {
		t.Fatalf("expected sequence increment (%d), got %d", id1+1, id2)
	}

	// 3. PollPendingCommands
	pending, err := backend.PollPendingCommands(ctx)
	if err != nil {
		t.Fatalf("PollPendingCommands failed: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending commands, got %d", len(pending))
	}
	if pending[0].ID != id1 || pending[1].ID != id2 {
		t.Fatalf("unexpected order: %d, %d", pending[0].ID, pending[1].ID)
	}

	// 4. UpdateCommandStatus for id1
	err = backend.UpdateCommandStatus(ctx, id1, "completed", map[string]interface{}{"msg_id": "ABC123XYZ"})
	if err != nil {
		t.Fatalf("UpdateCommandStatus failed: %v", err)
	}

	// 5. PollPendingCommands should now only return id2
	pendingAfter, err := backend.PollPendingCommands(ctx)
	if err != nil {
		t.Fatalf("PollPendingCommands after update failed: %v", err)
	}
	if len(pendingAfter) != 1 || pendingAfter[0].ID != id2 {
		t.Fatalf("expected only id2 pending, got %+v", pendingAfter)
	}

	// 6. WaitForCommand with asynchronous completion
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = backend.UpdateCommandStatus(context.Background(), id2, "completed", `{"status":"ok"}`)
	}()

	cmd2, err := backend.WaitForCommand(ctx, id2, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForCommand id2 failed: %v", err)
	}
	if cmd2.Status != "completed" {
		t.Fatalf("expected status completed, got %s", cmd2.Status)
	}
	var res map[string]string
	_ = json.Unmarshal(cmd2.Result, &res)
	if res["status"] != "ok" {
		t.Fatalf("unexpected result: %s", string(cmd2.Result))
	}

	// 7. WaitForCommand timeout
	id3, _ := backend.EnqueueCommand(ctx, "hang", nil)
	_, err = backend.WaitForCommand(ctx, id3, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error for WaitForCommand")
	}

	// 8. PutCommand and GetAllCommands
	customCmd := Command{
		ID:        999,
		Command:   "custom_sync",
		Payload:   json.RawMessage(`{"mode":"fast"}`),
		Status:    "completed",
		Result:    json.RawMessage(`{"records":5}`),
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt: time.Now().UTC(),
	}
	if err := backend.PutCommand(ctx, customCmd); err != nil {
		t.Fatalf("PutCommand failed: %v", err)
	}

	allCmds, err := backend.GetAllCommands(ctx)
	if err != nil {
		t.Fatalf("GetAllCommands failed: %v", err)
	}
	if len(allCmds) != 4 {
		t.Fatalf("expected 4 commands total, got %d", len(allCmds))
	}
	if allCmds[3].ID != 999 {
		t.Fatalf("expected last command to be ID 999, got %d", allCmds[3].ID)
	}

	// 9. ResetSequence
	if err := backend.ResetSequence(ctx); err != nil {
		t.Fatalf("ResetSequence failed: %v", err)
	}
	nextID, err := backend.EnqueueCommand(ctx, "after_reset", nil)
	if err != nil {
		t.Fatalf("EnqueueCommand after reset failed: %v", err)
	}
	if nextID <= 999 {
		t.Fatalf("expected sequence after reset to be > 999, got %d", nextID)
	}

	// 10. Verify changesets were captured
	if len(emittedChanges) < 4 {
		t.Fatalf("expected at least 4 changesets emitted, got %d", len(emittedChanges))
	}
}
