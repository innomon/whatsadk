# [whatsmeow fields containg senders mobile number](https://gemini.google.com/app/f3541b9df3ed178d)

In the **whatsmeow** Go library, the mobile number of a caller or sender is typically stored within a **JID** (Jabber ID) object. WhatsApp uses JIDs in the format `[phone_number]@[server]`.

### 1. Fields Containing the Mobile Number

To get the actual mobile number string, you look for the `User` field within a `types.JID` struct.

* **For Message Events (`events.Message`):**
* `evt.Info.Sender.User`: This is the phone number of the person who sent the message.
* `evt.Info.Chat.User`: In a 1-to-1 chat, this is the recipient's phone number. In a group chat, this is the group ID.


* **For Call Events (`events.CallOffer` or `events.UnknownCallEvent`):**
* `evt.BasicCallMeta.From.User`: This contains the phone number of the person initiating the call.
* `evt.CallCreator.User`: Available in some call-related metadata.



**Code Example:**

```go
func eventHandler(evt interface{}) {
    switch v := evt.(type) {
    case *events.Message:
        // The .User field contains the phone number (e.g., "1234567890")
        phoneNumber := v.Info.Sender.User
        fmt.Printf("Message from: %s\n", phoneNumber)
    case *events.CallOffer:
        callerNumber := v.BasicCallMeta.From.User
        fmt.Printf("Incoming call from: %s\n", callerNumber)
    }
}

```

---

### 2. Integration with Google ADK (Agent Development Kit) SSE

Integrating a Go-based WhatsApp client (`whatsmeow`) with Google ADK's Server-Sent Events (SSE) allows you to stream WhatsApp events directly to a web interface or an LLM-powered agent.

#### Architectural Summary

Google ADK usually runs as a Python/FastAPI backend. To connect it with `whatsmeow` (Go), you typically use a **Proxy-Subscriber** pattern.

1. **The Go Backend (whatsmeow):**
* Acts as the "Listener."
* When an event occurs (e.g., a message or call), the Go app captures it and sends a POST request to a "Webhook" endpoint on your Google ADK server.


2. **The Google ADK Server (Python/FastAPI):**
* Receives the webhook from Go.
* Uses a **Broadcaster** (like `asyncio.Queue` or a Redis Pub/Sub) to push this data to any active SSE clients.
* Exposes a `/run_sse` or custom `/stream` endpoint for the frontend.



#### Step-by-Step Integration

1. **Capture in Go:** In your `eventHandler`, serialize the caller/sender info into JSON and forward it to your ADK backend.
2. **ADK Webhook Receiver:** Create a FastAPI route in your ADK project to receive these events.
```python
@app.post("/whatsapp-webhook")
async def handle_webhook(data: dict):
    # Push 'data' into a global stream queue
    await stream_queue.put(data)
    return {"status": "ok"}

```


3. **SSE Streaming:**
Use ADK’s built-in SSE capabilities or a standard `EventSourceResponse` to stream the queue to your frontend.
```python
@app.get("/events")
async def message_stream():
    async def event_generator():
        while True:
            msg = await stream_queue.get()
            yield f"data: {json.dumps(msg)}\n\n"
    return EventSourceResponse(event_generator())

```


4. **Frontend/Agent Action:**
The frontend listens to `/events`. When a new caller ID is received via SSE, the Google ADK Agent can be triggered to lookup the contact in a CRM or provide a summarized greeting to the user.

## Fileter non +91 number

To filter messages in `whatsmeow` by country code, you need to inspect the `User` field of the sender's JID. In WhatsApp's infrastructure, the `User` portion of the JID is the phone number without the `+` sign.

Here is a clean implementation of an event handler that filters for the **91** (India) country code.

### Go Implementation: Country Code Filter

```go
package main

import (
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

func main() {
	// ... initialization code (client, connection, etc.) ...
}

func handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Extract the sender's JID
		senderJID := v.Info.Sender
		
		// The .User field is the phone number string (e.g., "919876543210")
		// WhatsApp JIDs do not include the leading '+'
		phoneNumber := senderJID.User

		// Filter: Check if the number starts with "91"
		if !strings.HasPrefix(phoneNumber, "91") {
			// Log and ignore numbers from outside India
			fmt.Printf("Filtered message from non-India number: %s\n", phoneNumber)
			return
		}

		// If it passes the filter, proceed with your logic
		fmt.Printf("Received message from India (%s): %s\n", phoneNumber, v.Message.GetConversation())
		
		// Proceed to send to your Google ADK SSE endpoint...
	}
}

```

---

### Key Technical Details

* **JID Structure:** The `v.Info.Sender` is a `types.JID` struct. The `User` field is a string containing the numeric digits of the phone number.
* **No "+" Sign:** Note that `whatsmeow` provides the raw user ID. While we colloquially say "+91", the actual data in the protocol is just `91...`.
* **The Filter Logic:** We use `strings.HasPrefix(phoneNumber, "91")`. This is more reliable than checking for a "+" which isn't present in the raw JID string.

### How this fits into the SSE flow

If you are integrating this with the Google ADK SSE setup discussed earlier, you would place this filter **before** you make the HTTP POST request to your Python/FastAPI backend. This saves bandwidth and prevents your LLM or Agent from processing international messages you aren't interested in.

---

**Would you like me to show you the Go code for the HTTP client part that sends this filtered data to your Google ADK backend?**
