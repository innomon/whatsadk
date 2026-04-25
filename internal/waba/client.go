package waba

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
	MessagingProduct string       `json:"messaging_product"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Text             *TextObject  `json:"text,omitempty"`
	Image            *ImageObject `json:"image,omitempty"`
}

type TextObject struct {
	Body string `json:"body"`
}

type ImageObject struct {
	ID      string `json:"id,omitempty"`
	Link    string `json:"link,omitempty"`
	Caption string `json:"caption,omitempty"`
}

type MediaResponse struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	FileSize int    `json:"file_size"`
	ID       string `json:"id"`
}

type UploadResponse struct {
	ID string `json:"id"`
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

func (c *Client) SendImage(ctx context.Context, to, mediaID, caption string) error {
	reqBody := MessageRequest{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "image",
		Image: &ImageObject{
			ID:      mediaID,
			Caption: caption,
		},
	}

	return c.send(ctx, reqBody)
}

func (c *Client) GetMediaURL(ctx context.Context, mediaID string) (*MediaResponse, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s", mediaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WABA Media Info API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var mediaResp MediaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mediaResp); err != nil {
		return nil, err
	}

	return &mediaResp, nil
}

func (c *Client) DownloadMedia(ctx context.Context, mediaURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WABA Media Download error (%d): %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}

// UploadMedia uploads binary data to Meta and returns the Media ID.
func (c *Client) UploadMedia(ctx context.Context, data []byte, fileName, mimeType string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/media", c.cfg.PhoneNumberID)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add messaging_product
	if err := writer.WriteField("messaging_product", "whatsapp"); err != nil {
		return "", err
	}

	// Create file part
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileName))
	h.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("WABA Media Upload error (%d): %s", resp.StatusCode, string(respBody))
	}

	var uploadResp UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return "", err
	}

	return uploadResp.ID, nil
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
