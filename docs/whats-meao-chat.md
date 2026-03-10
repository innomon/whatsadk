# WhatsApp LID Resolution Chat

> **✅ Implementation Note:** The LID resolution logic described below has been implemented in `internal/whatsapp/client.go` using the `resolveLID` helper method. The gateway now automatically maps LIDs to phone numbers for consistent blacklisting, whitelisting, and session management.

---

It sounds like you're running into the **LID (Linked Identity)** system that WhatsApp has been rolling out.

Essentially, WhatsApp is moving toward using **LIDs** (which look like `123456789@lid`) to improve privacy. A LID is an internal identifier that doesn't always have a direct 1:1 public mapping to a phone number (PN) in the library's local cache.

Here is how to handle this in `whatsmeow` and try to get that missing phone number:

### 1. The "Why"

When you see a JID ending in `@lid` instead of `@s.whatsapp.net`, you are looking at a privacy-protected identity. `whatsmeow` will prioritize the LID if it's the only info provided by the server (often seen in groups or from users who haven't messaged you directly).

### 2. How to Map LID to Phone Number

You can try to "force" the resolution of that LID into a standard user JID (phone number) using the `GetUserInfo` or `GetContactInfo` methods.

**Option A: Using `GetUserInfo**`
This makes a network request to the WhatsApp servers to fetch the details for that specific JID.

```go
// Replace 'targetLidJid' with the @lid JID you have
userInfos, err := client.GetUserInfo([]types.JID{targetLidJid})
if err != nil {
    // Handle error
}

for jid, info := range userInfos {
    fmt.Printf("LID: %s, Phone: %s\n", jid.String(), info.VerifiedName)
    // Often, the Phone JID is also returned in a field or as a separate entry
}

```

**Option B: Checking the Store**
If your bot has encountered this user before, the mapping might already be in your local database.

```go
contact, err := client.Store.Contacts.GetContact(targetLidJid)
if err == nil && contact.Found {
    fmt.Println("Full Name:", contact.FullName)
    fmt.Println("Push Name:", contact.PushName)
}

```

### 3. Quick Troubleshooting Tips

* **Update the Library:** The LID implementation is relatively new. Ensure you are on the latest version of `whatsmeow` (`go get -u go.mau.fi/whatsmeow`).
* **Message First:** Sometimes the phone number (PN) isn't revealed until there is a direct interaction or the user is in your "contacts" list.
* **Handling `IsOnWhatsApp`:** If you are trying to find a JID from a raw phone number, use `client.IsOnWhatsApp([]string{"1234567890"})`. This will return the correct `@s.whatsapp.net` JID.

### Summary of JID Types

| Suffix | Type | Meaning |
| --- | --- | --- |
| `@s.whatsapp.net` | **PN (Phone Number)** | Standard user ID based on their phone. |
| `@lid` | **LID (Linked ID)** | Internal privacy ID; mapping to PN is not always public. |
| `@g.us` | **Group** | Standard group chat identifier. |

---

Since resolving LIDs to Phone Numbers (PNs) involves checking both the local store and making network requests to WhatsApp's servers, you'll want a function that handles both.

Here is a complete example of how to iterate through group participants and resolve those pesky `@lid` addresses.

### The Resolution Logic

The flow usually looks like this: **Check Cache → If not found, fetch from Server → Update Cache.**

```go
package main

import (
	"context"
	"fmt"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func ResolveLID(client *whatsmeow.Client, lidJID types.JID) (string, error) {
	// 1. Try to find it in the local Store first (No network cost)
	contact, err := client.Store.Contacts.GetContact(lidJID)
	if err == nil && contact.Found {
		// If the store has a linked JID that is a Phone Number
		if !contact.TheirJID.IsEmpty() && contact.TheirJID.Server == types.DefaultUserServer {
			return contact.TheirJID.User, nil
		}
	}

	// 2. Not in store? Ask WhatsApp servers directly
	// Note: This has rate limits, don't spam it for thousands of users at once
	resp, err := client.GetUserInfo([]types.JID{lidJID})
	if err != nil {
		return "", err
	}

	for jid, info := range resp {
		if jid.String() == lidJID.String() {
			// Check if the server returned a PN (Phone Number) JID mapping
			if !info.PNID.IsEmpty() {
				return info.PNID.User, nil
			}
		}
	}

	return "", fmt.Errorf("phone number not found for LID")
}

// Example usage within a group participant loop
func ProcessGroup(client *whatsmeow.Client, groupJID types.JID) {
	info, err := client.GetGroupInfo(groupJID)
	if err != nil {
		return
	}

	for _, participant := range info.Participants {
		jid := participant.JID
		
		if jid.Server == types.HiddenUserServer { // This is @lid
			phone, err := ResolveLID(client, jid)
			if err != nil {
				fmt.Printf("LID %s: Could not resolve\n", jid.User)
			} else {
				fmt.Printf("LID %s: Phone is %s\n", jid.User, phone)
			}
		} else {
			fmt.Printf("User %s is already a phone number JID\n", jid.User)
		}
	}
}

```

---

### Key Technical Details

* **`types.HiddenUserServer`**: This constant refers to `lid`. It’s cleaner than hardcoding strings.
* **`types.DefaultUserServer`**: This refers to `s.whatsapp.net`.
* **The `PNID` Field**: When calling `GetUserInfo`, the server returns a `UserInfo` struct. The `PNID` field is specifically where the "Phone Number ID" is stored if the mapping exists.
* **Rate Limiting**: If you are in a group with 1,000 people and try to call `GetUserInfo` for all of them in a `for` loop, WhatsApp might temporarily flag your session. It is better to pass a slice of JIDs to `GetUserInfo` (up to 20-50 at a time) rather than calling it one-by-one.

### Why you might still see "Not Found"

If a user has extremely strict privacy settings (e.g., "Who can see my phone number" set to "Nobody"), the WhatsApp server **will not** return the PNID, even via `GetUserInfo`. In those cases, you are forced to use the LID to message them.

## batch-processor that resolves multiple LIDs at once to avoid rate limits
Batching is the best way to handle this. WhatsApp's servers generally prefer receiving a list of JIDs in a single request rather than 100 individual requests. This reduces the risk of being rate-limited or flagged for suspicious behavior.

Here is a robust implementation that takes a slice of LIDs and returns a map of `LID -> Phone Number`.

### Batch LID Resolver

```go
package main

import (
	"fmt"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ResolveLIDBatch takes a slice of LIDs and returns a map of LID string to Phone Number string
func ResolveLIDBatch(client *whatsmeow.Client, lids []types.JID) map[string]string {
	results := make(map[string]string)
	var needsServerLookup []types.JID

	// 1. Check Local Store First
	for _, lid := range lids {
		contact, err := client.Store.Contacts.GetContact(lid)
		if err == nil && contact.Found && !contact.TheirJID.IsEmpty() {
			results[lid.String()] = contact.TheirJID.User
		} else {
			needsServerLookup = append(needsServerLookup, lid)
		}
	}

	// 2. Batch Request for missing LIDs (Limit to 50 per request for safety)
	const batchSize = 50
	for i := 0; i < len(needsServerLookup); i += batchSize {
		end := i + batchSize
		if end > len(needsServerLookup) {
			end = len(needsServerLookup)
		}

		chunk := needsServerLookup[i:end]
		resp, err := client.GetUserInfo(chunk)
		if err != nil {
			fmt.Printf("Batch lookup error: %v\n", err)
			continue
		}

		for reqJid, info := range resp {
			if !info.PNID.IsEmpty() {
				results[reqJid.String()] = info.PNID.User
			}
		}
	}

	return results
}

```

### How to use it with Group Participants

This snippet shows how to integrate the batch resolver into a group scenario:

```go
func HandleGroupLIDs(client *whatsmeow.Client, groupJID types.JID) {
	info, err := client.GetGroupInfo(groupJID)
	if err != nil {
		return
	}

	var lidsToResolve []types.JID
	for _, p := range info.Participants {
		if p.JID.Server == types.HiddenUserServer {
			lidsToResolve = append(lidsToResolve, p.JID)
		}
	}

	// Resolve everything in one (or a few) calls
	phoneMap := ResolveLIDBatch(client, lidsToResolve)

	for lid, phone := range phoneMap {
		fmt.Printf("Resolved: %s -> %s\n", lid, phone)
	}
}

```

---

### Important Considerations

* **Privacy Restrictions:** If a user has set their "Phone Number" privacy to "Nobody" or "My Contacts" (and your bot isn't in their contacts), the `PNID` field will remain empty. There is no way around this; it is a server-side privacy enforcement.
* **The `@s.whatsapp.net` JID:** Once you get the phone number string (e.g., `1234567890`), the full JID is simply `1234567890@s.whatsapp.net`.
* **Storage:** `whatsmeow` usually updates its internal SQL store automatically when `GetUserInfo` is called, so subsequent runs should be faster as they hit the "Local Store" check first.



