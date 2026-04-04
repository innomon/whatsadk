# Media Bridge Implementation Plan

## Research & Preparation
- [ ] Verify `ffmpeg` availability in the execution environment.
- [ ] Review `whatsmeow` message handling logic for media events.
- [ ] Identify Go libraries for image resizing (e.g., `github.com/nfnt/resize` or standard `image` package).

## Phase 1: ADK Agent Client Upgrade
- [ ] Update `internal/agent/client.go`:
    - [ ] Add `InlineData` struct.
    - [ ] Update `Part` struct to include `InlineData`.
    - [ ] Add `ChatParts(ctx, userID, parts []Part)` method.
    - [ ] (Optional) Refactor `Chat(ctx, userID, message)` to use `ChatParts`.

## Phase 2: Media Processing Modules
- [ ] Create `internal/whatsapp/media.go`:
    - [ ] Implement `processImage(data []byte) ([]byte, error)` using `image` and `github.com/nfnt/resize`.
    - [ ] Implement `processAudio(data []byte) ([]byte, error)` using `ffmpeg` via `os/exec`.
    - [ ] Implement `processVideo(data []byte) ([][]byte, error)` using `ffmpeg` for 1 FPS sampling.
    - [ ] Implement `processDocument(data []byte, mimeType string) ([]Part, error)` for PDF/TXT/CSV.

## Phase 3: Integration
- [ ] Update `internal/whatsapp/client.go`'s `handleMessage` to process and forward media to the ADK agent.
- [ ] Ensure `Caption` is correctly extracted and sent as a `Text` part.
- [ ] Implement size and time limit guardrails.

## Phase 4: Testing & Validation
- [ ] **Unit Tests:** Test each media processing module with sample data.
- [ ] **Integration Tests:** Send various media types via WhatsApp and verify the ADK agent receives the correctly formatted parts.
- [ ] **End-to-End Test:** Confirm the agent can process the media and respond appropriately.

## Verification Checklist
- [ ] Image resized correctly to 896px.
- [ ] Audio converted to 16kHz PCM.
- [ ] Video frames sampled at 1 FPS.
- [ ] Multi-part message correctly formatted for ADK API.
- [ ] Error handling logs failures without crashing the service.
