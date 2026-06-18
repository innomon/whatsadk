# Router Example Implementation Plan

This plan details the steps to create a routing agent in `examples/router` that extends the `ignore` pattern with multi-app support and LLM-driven disambiguation.

## 1. Setup Files & Configuration
- Create `examples/router` directory.
- Define `examples/router/config.yaml` for standard ADK settings.
- Define `examples/router/router.yaml` for application mappings and classifier settings.
- Create a sample `examples/router/apps.json` to show the data format for `filesys`.

## 2. Core Agent Implementation (`main.go`)
- **Initialize Router**:
    - Load `router.yaml`.
    - Setup OpenAI client for the classifier.
- **Message Handler**:
    - Extract `userID` and `IsFromMe` flag from the ADK envelope/context.
    - Implement `resolveUserApps(userID, isMe)` function:
        - Read from `router/<userID>/apps.json`.
        - Inject `admin` and `su` if `isMe`.
        - Deduplicate and validate against `router.yaml` apps.
    - Implement `handleRouting(invCtx)`:
        - If 0 apps: Use `default_app`.
        - If 1 app: Proxy request to `a2aURL`.
        - If >1 app:
            - Check `filesys` for pending state.
            - If no state: Initiate "Selection Mode".
            - If pending: Resolve user input (Number, Title, or Classifier).

## 3. Storage Layer Integration
- Implement a helper to read/write `filesys` using the ADK `Session().Storage()` interface (or direct DB access if preferred for speed).
- Paths to use:
    - `router/<userID>/apps.json` (Read-only for router).
    - `router/<userID>/state.json` (Read-write for session persistence).

## 4. Classifier Integration
- Implement `classifyInput(text, titles)`:
    - Construct a prompt: "The user said '{text}'. Based on these titles: {titles}, return the index (1-N) of the most likely match. Return 0 if none match."
    - Call OpenAI API.
    - Parse integer response.

## 5. Proxy Logic
- Use Go's `net/http/httputil.NewSingleHostReverseProxy` or a manual HTTP forwarder to pass the ADK payload to the downstream agent.
- Ensure the `application/x-adk-silent-ignore` is returned if the selected app is `ignore`.

## 6. Verification & Testing
- Mock downstream agents (using `adksim` or `examples/hello`).
- Test "Me" identity auto-injection.
- Test manual selection via numbers and titles.
- Test LLM disambiguation with fuzzy inputs.
- Test fallback to `default_app` when no configuration exists.
