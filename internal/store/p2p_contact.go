package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func contactKey(ourJID, theirJID string) string {
	return fmt.Sprintf("whatsadk:contact:%s:%s", ourJID, theirJID)
}

// PutContact stores or updates a contact entry in crm_store.
func (b *Backend) PutContact(ctx context.Context, c Contact) error {
	rawMeta, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal contact: %w", err)
	}
	return b.putRecord(ctx, contactKey(c.OurJID, c.TheirJID), rawMeta, nil)
}

// ListContacts retrieves up to 100 contacts matching the query across full_name, push_name, or their_jid.
func (b *Backend) ListContacts(ctx context.Context, query string) ([]Contact, error) {
	var rows *sql.Rows
	var err error

	if query == "" {
		rows, err = b.db.QueryContext(ctx,
			"SELECT our_jid, their_jid, full_name, short_name, push_name, business_name FROM whatsmeow_contacts ORDER BY full_name ASC LIMIT 100",
		)
	} else {
		q := "%" + query + "%"
		rows, err = b.db.QueryContext(ctx,
			`SELECT our_jid, their_jid, full_name, short_name, push_name, business_name 
			 FROM whatsmeow_contacts 
			 WHERE full_name LIKE ? OR push_name LIKE ? OR their_jid LIKE ? 
			 ORDER BY full_name ASC LIMIT 100`,
			q, q, q,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var fullName, shortName, pushName, businessName sql.NullString
		if err := rows.Scan(&c.OurJID, &c.TheirJID, &fullName, &shortName, &pushName, &businessName); err != nil {
			return nil, fmt.Errorf("scan contact row: %w", err)
		}
		c.FullName = fullName.String
		c.ShortName = shortName.String
		c.PushName = pushName.String
		c.BusinessName = businessName.String
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// GetAllContacts retrieves all contacts ordered by full_name ascending.
func (b *Backend) GetAllContacts(ctx context.Context) ([]Contact, error) {
	rows, err := b.db.QueryContext(ctx,
		"SELECT our_jid, their_jid, full_name, short_name, push_name, business_name FROM whatsmeow_contacts ORDER BY full_name ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("get all contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var fullName, shortName, pushName, businessName sql.NullString
		if err := rows.Scan(&c.OurJID, &c.TheirJID, &fullName, &shortName, &pushName, &businessName); err != nil {
			return nil, fmt.Errorf("scan contact row: %w", err)
		}
		c.FullName = fullName.String
		c.ShortName = shortName.String
		c.PushName = pushName.String
		c.BusinessName = businessName.String
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}
