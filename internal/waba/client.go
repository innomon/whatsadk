package waba

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/innomon/whatsadk/internal/config"
)

type Client struct {
	cfg        *config.WABAConfig
	httpClient *http.Client
}

func NewClient(cfg *config.WABAConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type MessageRequest struct {
	MessagingProduct string      `json:"messaging_product"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             *TextObject `json:"text,omitempty"`
}

type TextObject struct {
	Body string `json:"body"`
}

func (c *Client) SendText(ctx context.Context, to, text string) error {
	reqBody := MessageRequest{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
		Text: &TextObject{
			Body: text,
		},
	}

	return c.send(ctx, reqBody)
}

func (c *Client) send(ctx context.Context, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", c.cfg.PhoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WABA API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// TODO: Implement SendMedia when needed, following the same pattern as whatsmeow client.
