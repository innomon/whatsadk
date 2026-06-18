# Specification: ADK ➔ WhatsApp Media Bridge

## 1. Objective
This specification defines how the `whatsadk` gateway transforms multi-part responses from a Google ADK agent (containing text and binary data) into WhatsApp messages via the `whatsmeow` library.

## 2. Technical Mapping

| ADK Part Type | MimeType | WhatsApp Message Type | Handling Logic |
| :--- | :--- | :--- | :--- |
| `text` | N/A | `Conversation` / `Caption` | Sent as a plain text message or used as a caption for a following media part. |
| `inlineData` | `image/jpeg`, `image/png` | `ImageMessage` | Uploaded to WhatsApp as an image. |
| `inlineData` | `audio/wav`, `audio/mp3`, `audio/ogg` | `AudioMessage` | Uploaded as audio. Note: Voice notes (PTT) require specific flags. |
| `inlineData` | `video/mp4` | `VideoMessage` | Uploaded as video. |
| `inlineData` | `application/pdf`, `text/*` | `DocumentMessage` | Uploaded as a document with a filename. |

## 3. Media Upload Process
WhatsApp requires media to be uploaded to their servers before a message referencing it can be sent. The gateway must perform the following steps for each `inlineData` part:

1.  **Decoding:** Convert the Base64 `data` string from ADK into raw bytes.
2.  **Upload:** Use the `whatsmeow` client's `Upload` method. This returns metadata:
    *   `URL`
    *   `DirectPath`
    *   `MediaKey`
    *   `FileSHA256`
    *   `FileEncSHA256`
    *   `FileLength`
3.  **Construction:** Populate the appropriate `waE2E.*Message` struct (e.g., `ImageMessage`) with the metadata.
4.  **Transmission:** Call `wac.SendMessage` with the constructed message.

## 4. Multi-Part Sequencing
ADK often returns a mixture of text and media. The gateway will handle these based on their order:

*   **Case: [Text, Media]** ➔ Send a single message of the Media type, using the Text as the `Caption`.
*   **Case: [Text, Media1, Media2]** ➔ Send Media1 with Text as `Caption`, then send Media2 as a separate message.
*   **Case: [Text1, Text2]** ➔ Concatenate into a single `Conversation` message.
*   **Case: [Media1, Media2]** ➔ Send two separate messages.

## 5. Constraints & Guardrails
*   **Size Limits:** Adhere to WhatsApp's file size limits (usually 16MB for video/audio, 100MB for documents). The gateway should reject ADK parts exceeding **20MB** by default to prevent memory exhaustion.
*   **MimeType Validation:** Only allow well-known types. Fallback to `DocumentMessage` for unknown binary types.
*   **Concurrency:** If a response contains multiple media parts, they should be uploaded and sent sequentially to preserve order and avoid overwhelming the connection.
