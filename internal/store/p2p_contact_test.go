package store

import (
	"context"
	"testing"

	"sqlite-p2p/pkg/p2p"
)

func TestContacts_PutListGetAll(t *testing.T) {
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

	c1 := Contact{
		OurJID:       "user1@s.whatsapp.net",
		TheirJID:     "1234567890@s.whatsapp.net",
		FullName:     "Bob Smith",
		ShortName:    "Bob",
		PushName:     "Bobby",
		BusinessName: "",
	}
	c2 := Contact{
		OurJID:       "user1@s.whatsapp.net",
		TheirJID:     "9876543210@s.whatsapp.net",
		FullName:     "Alice Wonderland",
		ShortName:    "Alice",
		PushName:     "Al",
		BusinessName: "Wonderland LLC",
	}

	// 1. Put Contacts
	if err := backend.PutContact(ctx, c1); err != nil {
		t.Fatalf("PutContact c1 failed: %v", err)
	}
	if err := backend.PutContact(ctx, c2); err != nil {
		t.Fatalf("PutContact c2 failed: %v", err)
	}

	// 2. GetAllContacts (should be ordered by full_name ASC: Alice then Bob)
	all, err := backend.GetAllContacts(ctx)
	if err != nil {
		t.Fatalf("GetAllContacts failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(all))
	}
	if all[0].FullName != "Alice Wonderland" || all[1].FullName != "Bob Smith" {
		t.Fatalf("unexpected order: %s, %s", all[0].FullName, all[1].FullName)
	}
	if all[0].BusinessName != "Wonderland LLC" {
		t.Fatalf("expected Wonderland LLC, got %s", all[0].BusinessName)
	}

	// 3. ListContacts without query
	listAll, err := backend.ListContacts(ctx, "")
	if err != nil {
		t.Fatalf("ListContacts empty query failed: %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(listAll))
	}

	// 4. ListContacts with query matching full_name
	searchAlice, err := backend.ListContacts(ctx, "Alice")
	if err != nil {
		t.Fatalf("ListContacts 'Alice' failed: %v", err)
	}
	if len(searchAlice) != 1 || searchAlice[0].TheirJID != "9876543210@s.whatsapp.net" {
		t.Fatalf("unexpected search results: %v", searchAlice)
	}

	// 5. ListContacts with query matching push_name
	searchBobby, err := backend.ListContacts(ctx, "Bobby")
	if err != nil {
		t.Fatalf("ListContacts 'Bobby' failed: %v", err)
	}
	if len(searchBobby) != 1 || searchBobby[0].TheirJID != "1234567890@s.whatsapp.net" {
		t.Fatalf("unexpected search results: %v", searchBobby)
	}

	// 6. Update Contact
	c1Updated := c1
	c1Updated.BusinessName = "Bob's Plumbing"
	if err := backend.PutContact(ctx, c1Updated); err != nil {
		t.Fatalf("PutContact update failed: %v", err)
	}

	allAfterUpdate, err := backend.GetAllContacts(ctx)
	if err != nil {
		t.Fatalf("GetAllContacts after update failed: %v", err)
	}
	if len(allAfterUpdate) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(allAfterUpdate))
	}
	var bobUpdated *Contact
	for _, c := range allAfterUpdate {
		if c.TheirJID == c1.TheirJID {
			bobUpdated = &c
			break
		}
	}
	if bobUpdated == nil || bobUpdated.BusinessName != "Bob's Plumbing" {
		t.Fatalf("contact was not updated properly: %+v", bobUpdated)
	}

	// 7. Verify changesets were captured
	if len(emittedChanges) < 3 {
		t.Fatalf("expected at least 3 changesets, got %d", len(emittedChanges))
	}
}
