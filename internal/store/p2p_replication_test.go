package store

import (
	"context"
	"testing"
	"time"

	"sqlite-p2p/pkg/p2p"
)

func TestP2P_ReplicationSimulation(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize two isolated backends (Node A and Node B)
	nodeA, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create nodeA: %v", err)
	}
	defer nodeA.Close()

	nodeB, err := NewBackend(Options{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create nodeB: %v", err)
	}
	defer nodeB.Close()

	// Wire simulated P2P network replication: Node A -> Node B
	nodeA.Tracker().Subscribe(func(cs *p2p.Changeset, raw []byte) {
		decoded, err := p2p.DecodeChangeset(raw)
		if err != nil {
			t.Errorf("failed to decode changeset: %v", err)
			return
		}
		if err := p2p.ApplyChangeset(ctx, nodeB.Repo(), decoded); err != nil {
			t.Errorf("failed to apply changeset to nodeB: %v", err)
		}
	})

	// Wire simulated P2P network replication: Node B -> Node A
	nodeB.Tracker().Subscribe(func(cs *p2p.Changeset, raw []byte) {
		decoded, err := p2p.DecodeChangeset(raw)
		if err != nil {
			t.Errorf("failed to decode changeset: %v", err)
			return
		}
		if err := p2p.ApplyChangeset(ctx, nodeA.Repo(), decoded); err != nil {
			t.Errorf("failed to apply changeset to nodeA: %v", err)
		}
	})

	// 2. Replicate Filesys (Text and Media)
	tsFile := time.Now().UTC()
	imgData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46} // Sample JPEG header
	err = nodeA.PutFile(ctx, "whatsmeow/123/img.jpg", map[string]interface{}{"mime_type": "image/jpeg"}, imgData, tsFile)
	if err != nil {
		t.Fatalf("nodeA PutFile failed: %v", err)
	}

	// Verify Node B received the file
	fileOnB, err := nodeB.GetFile(ctx, "whatsmeow/123/img.jpg")
	if err != nil {
		t.Fatalf("nodeB GetFile failed: %v", err)
	}
	if fileOnB == nil {
		t.Fatal("expected file to exist on nodeB")
	}
	if string(fileOnB.Content) != string(imgData) {
		t.Fatal("binary content mismatch on nodeB")
	}

	// 3. Replicate Contact from Node A -> Node B
	c1 := Contact{
		OurJID:       "agent@s.whatsapp.net",
		TheirJID:     "123@s.whatsapp.net",
		FullName:     "Alice P2P",
		ShortName:    "Alice",
		PushName:     "Al",
		BusinessName: "P2P Ventures",
	}
	if err := nodeA.PutContact(ctx, c1); err != nil {
		t.Fatalf("nodeA PutContact failed: %v", err)
	}

	contactsOnB, err := nodeB.GetAllContacts(ctx)
	if err != nil {
		t.Fatalf("nodeB GetAllContacts failed: %v", err)
	}
	if len(contactsOnB) != 1 || contactsOnB[0].FullName != "Alice P2P" {
		t.Fatalf("expected contact on nodeB, got: %+v", contactsOnB)
	}

	// 4. Replicate Blacklist from Node B -> Node A (bidirectional)
	if err := nodeB.AddBlacklist(ctx, "+555000", "P2P Spammer"); err != nil {
		t.Fatalf("nodeB AddBlacklist failed: %v", err)
	}

	isBlOnA, err := nodeA.IsBlacklisted(ctx, "+555000")
	if err != nil || !isBlOnA {
		t.Fatalf("expected phone blacklisted on nodeA: %v (err: %v)", isBlOnA, err)
	}

	// 5. Replicate Command Lifecycle
	cmdID, err := nodeA.EnqueueCommand(ctx, "sync_catalog", `{"depth":2}`)
	if err != nil {
		t.Fatalf("nodeA EnqueueCommand failed: %v", err)
	}

	// Verify command on Node B
	cmdsOnB, err := nodeB.GetAllCommands(ctx)
	if err != nil || len(cmdsOnB) != 1 {
		t.Fatalf("expected 1 command on nodeB, got %d (err: %v)", len(cmdsOnB), err)
	}
	if cmdsOnB[0].Status != "pending" {
		t.Fatalf("expected pending status on nodeB, got %s", cmdsOnB[0].Status)
	}

	// Update command status on Node A
	if err := nodeA.UpdateCommandStatus(ctx, cmdID, "completed", `{"items":42}`); err != nil {
		t.Fatalf("nodeA UpdateCommandStatus failed: %v", err)
	}

	// Verify update replicated to Node B
	cmdsOnBAfter, err := nodeB.GetAllCommands(ctx)
	if err != nil || len(cmdsOnBAfter) != 1 {
		t.Fatalf("expected 1 command on nodeB after update, got %v", cmdsOnBAfter)
	}
	if cmdsOnBAfter[0].Status != "completed" {
		t.Fatalf("expected completed status on nodeB, got %s", cmdsOnBAfter[0].Status)
	}

	// 6. Replicate Deletion (DeleteFile)
	if err := nodeA.DeleteFile(ctx, "whatsmeow/123/img.jpg"); err != nil {
		t.Fatalf("nodeA DeleteFile failed: %v", err)
	}
	deletedOnB, err := nodeB.GetFile(ctx, "whatsmeow/123/img.jpg")
	if err != nil {
		t.Fatalf("nodeB GetFile after delete failed: %v", err)
	}
	if deletedOnB != nil {
		t.Fatalf("expected deleted file to be nil on nodeB, got %+v", deletedOnB)
	}
}

func TestBackend_WithReplicationEngine(t *testing.T) {
	ctx := context.Background()
	db, err := p2p.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	repo := p2p.NewRepository(db)
	engine := p2p.NewReplicationEngine(repo, nil, nil)

	backend, err := NewBackend(Options{
		DB:     db,
		Repo:   repo,
		Engine: engine,
	})
	if err != nil {
		t.Fatalf("NewBackend with Engine failed: %v", err)
	}
	defer backend.Close()

	if err := backend.PutFile(ctx, "test/engine.txt", nil, []byte("via engine"), time.Now().UTC()); err != nil {
		t.Fatalf("PutFile via engine failed: %v", err)
	}
	if err := backend.DeleteFile(ctx, "test/engine.txt"); err != nil {
		t.Fatalf("DeleteFile via engine failed: %v", err)
	}
}
