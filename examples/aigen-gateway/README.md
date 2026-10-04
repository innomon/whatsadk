# WhatsADK to AIGenApp Agentic Gateway Integration Example

This example documents and illustrates how to configure and deploy the **WhatsADK Gateway** to connect directly to the **aigen-app** agentic application gateway.

Rather than running a standalone Router Agent (like `examples/router`), this integration maps the gateway directly to `aigen-app`'s multi-agent runtime. `aigen-app` verifies gateway authorization, resolves the WhatsApp user phone number to internal users and roles, filters sub-agents/extensions by RBAC permissions, and executes target agents in-process.

---

## 1. How It Works

```mermaid
sequenceDiagram
    actor User as WhatsApp User
    participant GW as WhatsADK Gateway
    participant AG as AIGenApp (adk2app REST /api/adk2app)
    participant R as RouterAgent (in-process)
    participant D as Downstream Agent (in-process)

    User->>GW: WhatsApp Message (Text/Media)
    Note over GW: Sign request with RS256 JWT using private key
    GW->>AG: POST /api/adk2app/run_sse (Target: RouterAgent)
    Note over AG: 1. Verify Gateway JWT with public key<br/>2. Resolve internal User & Roles via LoginByChannel<br/>3. Verify user has access to RouterAgent
    AG->>R: Execute RouterAgent Turn
    Note over R: 1. Retrieve user credentials from Context<br/>2. Load all sub-agents and app-extensions<br/>3. Filter targets: User must have read permission<br/>4. Determine target (direct route or selection menu)
    alt Multiple allowed apps (Ambiguous query)
        R-->>GW: SSE Response: Yield Choice Menu
        GW-->>User: Delivers reply via WhatsApp
    else Single allowed app (Or match resolved)
        R->>D: target.Run(ic)
        D-->>R: LLM Response Events
        R-->>GW: SSE Response: Yield LLM events
        GW-->>User: Delivers reply via WhatsApp
    end
```

---

## 2. Database Storage & File Locations

This integration uses the embedded `sqlite-p2p` storage engine (`sqlite-p2p://data/whatsadk_p2p.db?wal=true`):
- **Relative vs. Absolute Resolution:** Paths like `data/whatsadk_p2p.db` resolve relative to the current working directory from which the gateway process is run (e.g. `./data/whatsadk_p2p.db` from repository root).
- **Files Created:**
  - `data/whatsadk_p2p.db`: Primary SQLite database file for WhatsApp sessions and verification.
  - `data/whatsadk_p2p.db-wal`: Write-Ahead Log for concurrent transactions.
  - `data/whatsadk_p2p.db-shm`: Shared-Memory index.
- **Directory Creation:** The `data/` directory is automatically created on startup if absent.
- **P2P Replication:** The `p2p:` block enables decentralized peer discovery and replication over `whatsadk-mesh-topic`.

---

## 3. Configuration Steps

To link the gateway and `aigen-app` securely, you must configure public key cryptography and endpoints on both sides:

### Step A: Generate RSA JWT Keys
The gateway signs HTTP requests with an RS256 JWT, which `aigen-app` validates.
From the root of the `whatsadk` directory, run:
```bash
# Create keys directory
mkdir -p secrets

# Generate RSA private key
openssl genrsa -out secrets/jwt_private.pem 2048

# Derive the corresponding public key
openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
```

### Step B: Configure the WhatsADK Gateway
Update `examples/aigen-gateway/config.yaml`:
1. Point `adk.endpoint` to your `aigen-app` URL (e.g. `http://localhost:8080/api/adk2app`).
2. Set `adk.app_name` to `"RouterAgent"`.
3. Set `auth.jwt.private_key_path` to the private key path `secrets/jwt_private.pem` generated above.
4. Verify `whatsapp.store_dsn` and `verification.database_url` point to your `sqlite-p2p` database.

### Step C: Configure AIGenApp (`aigen-app`)
Provide the gateway's public key to `aigen-app` to allow validation:
1. Open `aigen-app`'s `config.yaml` configuration file.
2. Register the gateway's public key (contents of `secrets/jwt_public.pem` generated in Step A) under the WhatsApp channel configuration:
   ```yaml
   channels:
     whatsapp:
       public_key: |
         -----BEGIN PUBLIC KEY-----
         MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAv...
         -----END PUBLIC KEY-----
   ```

---

## 4. Running the Gateway

Once configuration is complete, run the WhatsADK Gateway targeting the example config:

```bash
# Start WhatsADK pointing to our configuration
./bin/gateway -config examples/aigen-gateway/config.yaml
```

---

## 5. Key Benefits of This Integration

- **Security & Authorization**: Rather than bypassing permissions during gateway entry, `aigen-app` performs entry-route RBAC checking. In-process `RouterAgent` execution then performs check filters dynamically so users can only view or execute authorized sub-agents/extensions.
- **Dynamic Selection (Selection Bypass)**: If a user only has permission to execute a single downstream agent, the selection menu is skipped and they are routed straight to that application seamlessly.
- **Zero Database Server Setup**: Uses embedded `sqlite-p2p` storage with automatic local directory initialization and optional mesh synchronization.
