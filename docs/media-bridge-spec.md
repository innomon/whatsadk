# Media Bridge Specification (whatsmeow ➔ Google ADK)

## 1. Scope and Objective
The Media Bridge is a component of the `whatsadk` gateway that intercepts incoming media messages from a `whatsmeow` WhatsApp client, processes them according to the requirements of the Google ADK (Agent Development Kit), and forwards them as structured parts within a session.

## 2. Technical Mapping

| Media Category | WhatsApp Source (`whatsmeow`) | Required ADK Format / Standard | ADK Part Type |
| :--- | :--- | :--- | :--- |
| **Image** | JPEG / PNG | **JPEG** (Normalized to 896x896) | `inlineData` (image/jpeg) |
| **Audio** | **OGG/Opus** | **PCM 16-bit Mono** (16kHz) | `inlineData` (audio/wav) |
| **Video** | MP4 (H.264) | **Sampled Frames** (1 FPS) | Multiple `inlineData` (image/jpeg) |
| **Text** | Plain Text | **UTF-8** | `text` |

## 3. Functional Requirements

### A. Image Normalization Module
* **Resizing:** For images > 1.5MB or with any side > 2000px, resize the shortest side to **896px** while maintaining aspect ratio.
* **Format Conversion:** Convert all incoming images (PNG, WebP, etc.) to **JPEG**.
* **ADK Packaging:** Wrap the resulting bytes in a base64-encoded `inlineData` part.
* **Caption:** If the WhatsApp message contains a caption, it must be sent as a preceding `text` part in the same `Message`.

### B. Audio Transcoding Module
* **Transcoding:** Convert `OGG/Opus` to `PCM 16-bit 16kHz Mono`.
* **Container:** Wrap the raw PCM in a **WAV** header for broad compatibility.
* **Detection:** Check `Ptt: true` in `AudioMessage` to flag as voice note (Push-to-Talk).
* **ADK Packaging:** Send as `inlineData` with `mimeType: "audio/wav"`.

### C. Video Sampling Module
* **Extraction:** Sample one frame per second from the video.
* **Format:** Each frame must be a **JPEG** image (normalized to 896px if necessary).
* **Payload:** Send sampled frames as a sequence of `inlineData` parts. 
* **Limit:** Maximum 20 sampled frames to stay within context limits.

### D. Document Handling Module
* **Direct Support:** PDF and plain text files (`.txt`, `.csv`) are passed directly as `inlineData` with their respective MIME types.
* **Extraction:** For text-based documents, the bridge may optionally extract the text and send it as a `text` part if the file size is small (< 32KB).
* **Office Documents:** DOCX and XLSX are not natively supported by all LLM backends. The bridge should either skip them with a message or convert them to **PDF** using a system tool (e.g., `unoconv` or `pandoc`).
* **Metadata:** Always include the `FileName` and `Caption` (if available) as a preceding `text` part.

## 4. Implementation Logic (Go)

The `internal/agent/client.go` will be updated to handle multi-part messages:

```go
type Part struct {
    Text       string      `json:"text,omitempty"`
    InlineData *InlineData `json:"inlineData,omitempty"`
}

type InlineData struct {
    MimeType string `json:"mimeType"`
    Data     string `json:"data"` // base64
}
```

## 5. Error Handling & Guardrails
* **Timeout:** Media processing (including download and transcoding) must complete within **60 seconds**.
* **Size Limit:** Reject media files larger than **20MB**.
* **Fallback:** If processing fails, log the error and optionally send the raw media or skip it to avoid crashing the gateway.

## 6. Dependencies
* `google.golang.org/adk`: ADK framework.
* `go.mau.fi/whatsmeow`: WhatsApp client.
* `ffmpeg`: System dependency for audio/video processing.
