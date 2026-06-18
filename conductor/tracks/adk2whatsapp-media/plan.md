# Implementation Plan: ADK ➔ WhatsApp Media Support

This plan outlines the steps to enable multi-media responses from ADK to be delivered to WhatsApp users.

## Phase 1: Agent Client Updates (`internal/agent`)

### 1.1 Update Response Extraction
*   Modify `extractFinalResponse` in `client.go` to return `[]Part` instead of `string`.
*   Rename it to `extractFinalParts` to reflect its new purpose.
*   Ensure it captures all parts (both `Text` and `InlineData`) from the model's message.

### 1.2 Update Public API
*   Change `Chat` and `ChatParts` signatures to return `([]Part, error)`.
*   This allows the WhatsApp client to receive the full structured response.

## Phase 2: WhatsApp Client Updates (`internal/whatsapp`)

### 2.1 Implement Media Upload Helper
*   Add a method `uploadMedia(ctx, data []byte, appMime whatsmeow.MediaType) (*waE2E.*Message, error)` to `client.go`.
*   This method will:
    1.  Call `c.wac.Upload(ctx, data, appMime)`.
    2.  Fill in the common fields (URL, DirectPath, MediaKey, etc.).
    3.  Return the specific message struct (ImageMessage, AudioMessage, etc.).

### 2.2 Implement Part Processing Logic
*   Add a method `sendADKParts(ctx, chat types.JID, parts []agent.Part)` to `client.go`.
*   Implement the sequencing logic defined in the specification:
    *   Buffer leading text to use as a caption for the first media part.
    *   If no media part follows, send text as a `Conversation`.
    *   Handle subsequent media parts as individual messages.

### 2.3 Wiring in `handleMessage`
*   Update the call to `c.adkClient.ChatParts` to handle the `[]agent.Part` return value.
*   Replace the existing `SendMessage` call (which only handled text) with a call to `sendADKParts`.

## Phase 3: Validation & Testing

### 3.1 Unit Tests
*   Add tests in `internal/agent/client_test.go` (if it exists, otherwise create it) to verify `extractFinalParts` correctly handles mixed content.
*   Add tests in `internal/whatsapp/client_test.go` to mock media uploads and verify message construction.

### 3.2 Integration Testing
*   Use a test ADK agent that returns an image and text.
*   Verify the WhatsApp user receives the image with the correct caption.

## Phase 4: Documentation
*   Update `README.md` and `ARCHITECTURE.md` to reflect that the gateway now supports outbound multi-media.
