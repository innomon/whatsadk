# Specification: ADK Silent Ignore Message

## Objective
Allow an ADK agent to instruct the WhatsApp gateway to "silently ignore" a user message. This means the gateway will not send any reply back to WhatsApp, but it will record that the message was ignored and the reason for doing so.

## Mechanism
The signal is sent as a special part in the ADK response message.

## Message Format
The ADK response should include an `InlineData` part with the following properties:

- **MimeType**: `application/x-adk-silent-ignore`
- **Data**: A Base64-encoded string representing the reason for ignoring the message.

### Example JSON (ADK Response)
```json
{
  "parts": [
    {
      "inlineData": {
        "mimeType": "application/x-adk-silent-ignore",
        "data": "VXNlciBpcyBvZmYtdG9waWM=" 
      }
    }
  ]
}
```
*(Data decoded: "User is off-topic")*

## Gateway Behavior
When the gateway receives a response from ADK containing this special part:
1. It extracts the reason from the `Data` field.
2. It logs the event: `Silently ignoring message from {userID}. Reason: {reason}`.
3. It records the ignore event in the internal store (filesys) as a "response" with metadata indicating it was ignored.
4. It **stops** further processing of the ADK response and does **not** send any message to the user via WhatsApp.

## Recording (Store)
- **Path**: `whatsmeow/{userID}/{uniqueID}/response`
- **Content**: The reason string.
- **Metadata**: 
  - `mime_type`: `text/plain`
  - `metadata.error`: `Ignored: {reason}`

---

# Implementation Plan

## 1. Define Constant
Add `MimeTypeSilentIgnore` constant to `internal/agent/client.go` to avoid magic strings.

## 2. Implement Detection Logic
Update `sendADKParts` in `internal/whatsapp/client.go` to scan for the silent ignore part before sending any messages.

## 3. Implement Early Exit and Storage
In `sendADKParts`, if the ignore part is detected:
- Decode the reason.
- Call `storeResponse` with the "Ignored" error string.
- Return immediately to prevent sending other parts.

## 4. Verification
- Add a unit test to verify the logic.
- Test with a mock ADK response containing the silent ignore part.

## Checklist
- [x] Define `MimeTypeSilentIgnore` in `internal/agent/client.go`.
- [x] Update `internal/whatsapp/client.go` with silent ignore logic.
- [x] Create specification documentation (`docs/adk-silent-ignore-spec.md`).
- [ ] Add unit tests in `internal/whatsapp/silent_ignore_test.go`.
- [ ] Update `README.md` with the new capability.
