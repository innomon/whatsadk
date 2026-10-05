# Media Bridge Implementation Plan

## Research & Preparation
- [x] Verify `ffmpeg` availability in the execution environment.
- [x] Review `whatsmeow` message handling logic for media events.
- [x] Identify Go libraries for image resizing (e.g., `github.com/nfnt/resize` or standard `image` package).

## Phase 1: ADK Agent Client Upgrade
- [x] Update `internal/agent/client.go`:
    - [x] Add `InlineData` struct.
    - [x] Update `Part` struct to include `InlineData`.
    - [x] Add `ChatParts(ctx, userID, parts []Part)` method.
    - [x] Refactor `Chat(ctx, userID, message)` to use `ChatParts`.

## Phase 2: Media Processing Modules
- [x] Create `internal/whatsapp/media.go`:
    - [x] Implement `ProcessImage(ctx, data []byte)` using `image` and `github.com/nfnt/resize`.
    - [x] Implement `ProcessAudio(ctx, data []byte)` using `ffmpeg` via `os/exec`.
    - [x] Implement `ProcessVideo(ctx, data []byte)` using `ffmpeg` for 1 FPS sampling.
    - [x] Implement `ProcessDocument(ctx, data []byte, mimeType string)` for PDF/TXT/CSV.

## Phase 3: Integration
- [x] Update `internal/whatsapp/client.go`'s `handleMessage` to process and forward media to the ADK agent.
- [x] Ensure `Caption` is correctly extracted and sent as a `Text` part.
- [x] Implement size and time limit guardrails.

## Phase 4: Testing & Validation
- [x] **Unit Tests:** Test each media processing module with sample data in `internal/whatsapp/media_test.go`.
- [ ] **Integration Tests:** Send various media types via WhatsApp and verify the ADK agent receives the correctly formatted parts.
- [ ] **End-to-End Test:** Confirm the agent can process the media and respond appropriately.

## Verification Checklist
- [x] Image resized correctly to 896px.
- [x] Audio converted to 16kHz PCM.
- [x] Video frames sampled at 1 FPS.
- [x] Multi-part message correctly formatted for ADK API.
- [x] Error handling logs failures without crashing the service.
