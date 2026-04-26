# ADK Router Example

This example demonstrates a sophisticated **Router Agent** that acts as a gateway to multiple other ADK agents. It can route user requests based on state, user preferences, and even use an LLM-based classifier to determine the best target agent.

## Features

- **Dynamic Routing**: Routes requests to different backend agents based on user session state.
- **In-Process Execution**: Supports running agents in-process using `agentic` configurations for better performance (no network hop).
- **Remote Routing**: Supports routing to remote agents via HTTP (A2A protocol).
- **LLM Classifier**: Uses an LLM to help identify which application the user wants to use when the input is ambiguous.
- **State Management**: Uses a PostgreSQL store to keep track of user-selected applications and session history.
- **Automatic Selection**: If only one application is available to a user, it routes directly. If multiple are available, it prompts for selection.

## Configuration

The router is configured via `router.yaml`.

### Example `router.yaml`

```yaml
default_app: "ignore"
pgsql_url: "postgres://user:pass@localhost:5432/whatsadk?sslmode=disable"

apps:
  - appName: "admin"
    a2aURL: "http://localhost:8081"
    adkAppName: "admin-agent"
    title: "Admin Tools"
  - appName: "shopper"
    agenticConfig: "shopper-agentic.yaml"
    adkAppName: "shopper-agent"
    title: "Shopping Assistant"

classifier:
  provider: "ollama"
  model: "gemma2"
  endpoint: "http://localhost:11434/v1"
  api_key: "none"
  prompt: "The user said: '${text}'. Which of the following options does this match best? ${optios}. Return the index number (1-N). Return 0 if none match."
  fallback_message: "Sorry, I couldn't identify the application. Please reply with the number or title."

prompts:
  selection: "I found multiple applications for you. Which one would you like to use?\n\n${optios}\n\nPlease reply with a number or name."
```

### App Configuration Types

1.  **Remote Agents**: Defined using `a2aURL`. The router will call these agents over HTTP.
2.  **In-Process Agents**: Defined using `agenticConfig`. The router will load the agent defined in the provided `agentic` YAML file and execute it in-process using `runner.Runner`.

## Agentic Configuration (`shopper-agentic.yaml`)

For in-process agents, you provide a standard `agentic` configuration file:

```yaml
root_agent: shopper-agent

models:
  gemini-flash:
    provider: gemini
    model_id: gemini-2.0-flash-exp
    default: true

agents:
  shopper-agent:
    description: A simple shopping assistant.
    model: gemini-flash
    instruction: |
      You are a shopping assistant. help the user find products.
```

## How it Works

1.  **Identity**: The router identifies the user via their `UserID`.
2.  **App Discovery**: It checks the database for a list of apps allowed for that user (`router/<userID>/apps.json`).
3.  **Routing Logic**:
    - If a single app is found, it routes directly.
    - If multiple apps are found, it checks `router/<userID>/state.json` to see if a selection is pending.
    - If no selection is pending, it sends a selection menu.
    - Once an app is selected (either by index, title, or LLM classification), the router executes the target app.
4.  **Execution**:
    - If the target app has an `agenticConfig`, the router uses an internal `RunnerManager` to execute the agent in the same process.
    - Otherwise, it uses an HTTP client to forward the request to the `a2aURL`.

## Running the Example

1.  Ensure you have a PostgreSQL database running and update the `pgsql_url` in `router.yaml`.
2.  Build the example:
    ```bash
    go build -o router_example main.go
    ```
3.  Run the router:
    ```bash
    ./router_example
    ```
