# Router Agent Specification

The `router` agent acts as a traffic controller between the WhatsApp Gateway and multiple ADK applications (A2A). It determines the appropriate target application based on a user's provisioned application list, sender identity, and optionally, an LLM-based classifier for disambiguation.

## Architecture

```
WhatsApp Gateway <---> Router Agent <---> [ Downstream ADK Apps ]
                                 \
                                  +---> Classifier (OpenAI)
                                 \
                                  +---> Storage (filesys)
```

## Configuration

The agent uses a standard `config.yaml` for its ADK port and a specialized `router.yaml` for routing logic.

### `router.yaml` Schema

```yaml
default_app: "ignore" # Fallback if no apps are found or default is needed
pgsql_url: "postgres://user:pass@localhost:5432/whatsadk?sslmode=disable" # If provided, use filesys table directly

apps:
  - appName: "admin"
    a2aURL: "http://localhost:8081"
    adkAppName: "admin-agent"
    title: "Admin Tools"
  - appName: "shopper"
    a2aURL: "http://localhost:8082"
    adkAppName: "shopper-agent"
    title: "Shopping Assistant"
  - appName: "laundry"
    a2aURL: "http://localhost:8083"
    adkAppName: "laundry-agent"
    title: "Laundry Service"
  - appName: "health"
    a2aURL: "http://localhost:8084"
    adkAppName: "health-agent"
    title: "Health Monitor"

classifier:
  provider: "openai"
  model: "gpt-4"
  endpoint: "https://api.openai.com/v1"
  api_key: "${OPENAI_API_KEY}" # Loaded from env
  prompt: "The user said: '${text}'. Which of the following options does this match best? ${options}. Return the index number (1-N). Return 0 if none match."
  fallback_message: "Sorry, I couldn't identify the application. Please press the number or type the exact title."

prompts:
  selection: "I found multiple applications for you. Which one would you like to use?\n\n${options}\n\nPlease reply with a number or name."
```

## Routing Logic

### 1. Identify User and App List
- **User ID**: Extracted from `invCtx.Session().UserID()`.
- **Apps Source**:
    - **Filesys**: Check `router/<userID>/apps.json` (JSON array of `appName`).
    - **Me Check**: If `msg.Info.IsFromMe` is true, automatically append `["admin", "su"]` to the list.
- **Resulting List**:
    - If empty: Use `default_app`.
    - If `default_app == "ignore"`: Send `application/x-adk-silent-ignore`.

### 2. Resolution Strategy
- **Single App**: Immediately route to the target `a2aURL`.
- **Multiple Apps**:
    - **State Check**: Check if the user is in a "Pending Choice" state (tracked in `filesys` at `router/<userID>/state.json`).
    - **New Session**:
        - Generate selection message:
          1. Admin Tools
          2. Shopping Assistant
          ...
        - Store the apps list in `state.json`.
        - Send selection message back to user.
    - **Existing Session (Selection Input)**:
        - **Number Match**: If input is "1", "2", etc., map to the corresponding app from `state.json`.
        - **Title Match**: If input exactly matches a `title`, map to that app.
        - **Classifier Disambiguation**:
            - Call OpenAI with the user's text and the available titles.
            - Classifier returns an index (0 for "none", 1+ for matches).
            - If index 0: Send `fallback_message`.
            - If index > 0: Route to the mapped app.

### 3. Routing Execution
- The router agent functions as a proxy.
- It forwards the incoming request to the target `a2aURL` and returns the response back to the gateway.
- If the target is `ignore`, it returns the silent ignore blob.

## Storage Integration (`filesys`)

The agent interacts with the gateway's `filesys` table via the ADK session interface or direct database access (if configured).
- `router/<userID>/apps.json`: Provisioned app list.
- `router/<userID>/state.json`: Current routing state (e.g., `{"status": "pending_selection", "options": ["admin", "shopper"]}`).
