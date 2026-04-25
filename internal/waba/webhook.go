package waba

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/config"
)

type WebhookHandler struct {
	cfg            *config.WABAConfig
	onMessage      func(sender, text string)
	onMessageParts func(sender string, parts []agent.Part)
}

func NewWebhookHandler(cfg *config.WABAConfig, onMessage func(sender, text string)) *WebhookHandler {
	return &WebhookHandler{
		cfg:       cfg,
		onMessage: onMessage,
	}
}

func (h *WebhookHandler) SetOnMessageParts(fn func(sender string, parts []agent.Part)) {
	h.onMessageParts = fn
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.handleVerification(w, r)
		return
	}

	if r.Method == http.MethodPost {
		h.handleNotification(w, r)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (h *WebhookHandler) handleVerification(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == h.cfg.VerifyToken {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}

	w.WriteHeader(http.StatusForbidden)
}

func (h *WebhookHandler) handleNotification(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Validate Signature
	if h.cfg.AppSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !h.validateSignature(body, sig) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Value.Messages != nil {
				for _, msg := range change.Value.Messages {
					if h.onMessageParts != nil {
						parts := h.parseMessageParts(msg)
						if len(parts) > 0 {
							h.onMessageParts(msg.From, parts)
						}
					} else if msg.Type == "text" && msg.Text != nil {
						h.onMessage(msg.From, msg.Text.Body)
					}
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) parseMessageParts(msg struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      *struct {
		Body string `json:"body"`
	} `json:"text,omitempty"`
	Image *struct {
		ID       string `json:"id"`
		MimeType string `json:"mime_type"`
		Caption  string `json:"caption,omitempty"`
	} `json:"image,omitempty"`
}) []agent.Part {
	var parts []agent.Part
	switch msg.Type {
	case "text":
		if msg.Text != nil {
			parts = append(parts, agent.Part{Text: msg.Text.Body})
		}
	case "image":
		if msg.Image != nil {
			// We send the caption as a text part first
			if msg.Image.Caption != "" {
				parts = append(parts, agent.Part{Text: msg.Image.Caption})
			}
			// We send a special part containing the Media ID
			// The main loop in waba-gateway will handle downloading this
			parts = append(parts, agent.Part{
				InlineData: &agent.InlineData{
					MimeType: msg.Image.MimeType,
					Data:     "media_id:" + msg.Image.ID, // Custom protocol to communicate with gateway
				},
			})
		}
	}
	return parts
}

func (h *WebhookHandler) validateSignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	actualSig := signature[7:]

	mac := hmac.New(sha256.New, []byte(h.cfg.AppSecret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(actualSig), []byte(expectedSig))
}

// WebhookPayload represents the Meta Webhook structure
type WebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Text      *struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
					Image *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						Caption  string `json:"caption,omitempty"`
					} `json:"image,omitempty"`
				} `json:"messages"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}
