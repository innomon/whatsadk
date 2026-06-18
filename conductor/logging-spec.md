# Specification: Structured Logging and Log Rotation

## 1. Objective
Improve the observability and traceability of WhatsADK. The logger must:
1. Print human-readable logs to the console (`stdout` / `stderr`).
2. Append structured logs in JSON Lines (JSONL) format to a log file.
3. Include emitting source file location (filename and line number) in the JSONL logs so external agents can trace messages back to the source code.
4. Manage log file sizes via a built-in log rotation mechanism.
5. Provide configuration via the YAML configuration file and environment variables.

---

## 2. Configuration
We will introduce a `logging` section under the YAML configuration schema:

```yaml
logging:
  level: "INFO"             # DEBUG, INFO, WARN, ERROR
  console_enabled: true     # Enable/disable console logging
  file_enabled: true        # Enable/disable file logging
  dir: "logs"               # Directory path for log files
  file_name: "whatsadk.log" # Log filename
  max_size_mb: 10          # Max size of log file before rotation
  max_backups: 5            # Number of old log files to retain
```

### Environment Overrides:
* `LOG_LEVEL`
* `LOG_DIR`
* `LOG_FILE_NAME`
* `LOG_MAX_SIZE_MB`
* `LOG_MAX_BACKUPS`

---

## 3. Log Output & Formatting

### 3.1 Console Handler
* Prints human-readable text.
* Uses standard timestamp, log level, and the message string.

### 3.2 JSONL File Handler
* Encodes records as JSON Lines.
* Format must include the following keys:
  * `time`: RFC3339 timestamp.
  * `level`: Log level (e.g. `INFO`, `ERROR`).
  * `msg`: The log message.
  * `source`: Object containing:
    * `file`: Path to the Go file emitting the log (e.g., `internal/whatsapp/client.go`).
    * `line`: Emitting source line number.
    * `function`: Function name.

---

## 4. Log Rotation Design (`LogRotator`)
To avoid external dependencies and keep the gateway lightweight, we will implement a handcrafted, thread-safe `LogRotator` that implements `io.Writer`.

### Logic:
1. **Write**: Acquire mutex. Check if writing the current slice `p` would exceed `max_size_mb * 1024 * 1024` bytes.
2. **Rotate**: If limit is exceeded, close the current file. Rename the current file to `filename.YYYYMMDD-HHMMSS.log`. Open a new file under `filename`.
3. **Prune**: Scan the log directory for backups matching the prefix, sort by age, and delete any files exceeding `max_backups`.

---

## 5. Integration

### 5.1 Package `internal/logger`
A new package exposing:
* `Init(cfg *config.Config) (*slog.Logger, error)`: Set up the global/custom slog logger.
* `NewWhatsMeowLogger(s *slog.Logger) waLog.Logger`: A wrapper translating slog logs into the `whatsmeow` logger interface.

### 5.2 Client Integrations
* **Multi-Device Gateway (`cmd/gateway/main.go`)**: Initialize and set as global logger.
* **WABA Gateway (`cmd/waba-gateway/main.go`)**: Initialize and use throughout the request processing loop.
* **Verification Handler (`internal/verification/handler.go`)**: Replace local slog instances with the configured unified logger.
