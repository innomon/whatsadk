# WhatsADK - WhatsApp to ADK Gateway

A Go utility that connects WhatsApp via QR code and proxies messages to a Google ADK Agent.

## Features

- Connect to WhatsApp using QR code scanning
- Persistent session storage (SQLite)
- Proxy incoming WhatsApp messages to ADK Agent
- Send agent responses back to WhatsApp users
- Configurable via YAML file or environment variables

## Requirements

- Go 1.23+
- Google API Key for Gemini

## Installation

```bash
go build -o whatsadk ./cmd/gateway
```

## Configuration

Set the Google API Key:
```bash
export GOOGLE_API_KEY=your-api-key
```

Optionally, customize `config/config.yaml`:
```yaml
whatsapp:
  store_path: "whatsapp.db"
  log_level: "INFO"

agent:
  name: "whatsapp_assistant"
  description: "A helpful WhatsApp assistant"
  instruction: "You are a helpful AI assistant responding via WhatsApp."
  model: "gemini-2.5-flash"
```

## Usage

```bash
./whatsadk
```

On first run, scan the QR code with WhatsApp:
1. Open WhatsApp on your phone
2. Go to Settings > Linked Devices
3. Tap "Link a Device"
4. Scan the QR code displayed in terminal

Once linked, the gateway will:
- Receive messages from WhatsApp contacts
- Forward them to the ADK agent
- Send agent responses back to the sender

## Architecture

```
┌──────────────┐     ┌─────────────────┐     ┌───────────────┐
│  WhatsApp    │────▶│  WhatsADK       │────▶│  Google ADK   │
│  (Phone)     │◀────│  Gateway        │◀────│  Agent        │
└──────────────┘     └─────────────────┘     └───────────────┘
                            │
                     ┌──────┴──────┐
                     │  SQLite DB   │
                     │ (sessions)   │
                     └─────────────┘
```

## License

MIT
