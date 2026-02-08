package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
)

type Client struct {
	endpoint   string
	appName    string
	apiKey     string
	streaming  bool
	httpClient *http.Client
	jwtGen     *auth.JWTGenerator
}

type RunRequest struct {
	AppName    string   `json:"appName"`
	UserID     string   `json:"userId"`
	SessionID  string   `json:"sessionId"`
	NewMessage *Message `json:"newMessage"`
	Streaming  bool     `json:"streaming,omitempty"`
}

type Message struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text,omitempty"`
}

type SessionRequest struct {
	State map[string]any `json:"state,omitempty"`
}

type Event struct {
	Content *Content `json:"content,omitempty"`
	Author  string   `json:"author,omitempty"`
	Partial bool     `json:"partial,omitempty"`
}

type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts,omitempty"`
}

func NewClient(cfg *config.ADKConfig, jwtGen *auth.JWTGenerator) *Client {
	return &Client{
		endpoint:  strings.TrimSuffix(cfg.Endpoint, "/"),
		appName:   cfg.AppName,
		apiKey:    cfg.APIKey,
		streaming: cfg.Streaming,
		jwtGen:    jwtGen,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) EnsureSession(ctx context.Context, userID string) error {
	sessionID := userID
	url := fmt.Sprintf("%s/apps/%s/users/%s/sessions/%s", c.endpoint, c.appName, userID, sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to create session request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if err := c.addAuthHeader(req, userID); err != nil {
		return fmt.Errorf("failed to set auth header: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "already exists") {
			return nil
		}
		return fmt.Errorf("session creation failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) Chat(ctx context.Context, userID, message string) (string, error) {
	if err := c.EnsureSession(ctx, userID); err != nil {
		return "", err
	}

	if c.streaming {
		return c.chatSSE(ctx, userID, message)
	}
	return c.chatRun(ctx, userID, message)
}

func (c *Client) chatRun(ctx context.Context, userID, message string) (string, error) {
	runReq := RunRequest{
		AppName:   c.appName,
		UserID:    userID,
		SessionID: userID,
		NewMessage: &Message{
			Role: "user",
			Parts: []Part{
				{Text: message},
			},
		},
	}

	body, err := json.Marshal(runReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/run", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if err := c.addAuthHeader(req, userID); err != nil {
		return "", fmt.Errorf("failed to set auth header: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("run failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var events []Event
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return extractFinalResponse(events), nil
}

func (c *Client) chatSSE(ctx context.Context, userID, message string) (string, error) {
	runReq := RunRequest{
		AppName:   c.appName,
		UserID:    userID,
		SessionID: userID,
		NewMessage: &Message{
			Role: "user",
			Parts: []Part{
				{Text: message},
			},
		},
		Streaming: true,
	}

	body, err := json.Marshal(runReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/run_sse", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if err := c.addAuthHeader(req, userID); err != nil {
		return "", fmt.Errorf("failed to set auth header: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("run_sse failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var events []Event
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event Event
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading SSE stream: %w", err)
	}

	return extractFinalResponse(events), nil
}

func (c *Client) addAuthHeader(req *http.Request, userID string) error {
	if c.jwtGen != nil {
		token, err := c.jwtGen.Token(userID)
		if err != nil {
			return fmt.Errorf("failed to generate JWT: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return nil
}

func extractFinalResponse(events []Event) string {
	var result strings.Builder

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Content != nil && event.Content.Role == "model" && !event.Partial {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					result.WriteString(part.Text)
				}
			}
			if result.Len() > 0 {
				break
			}
		}
	}

	if result.Len() == 0 {
		for _, event := range events {
			if event.Content != nil && event.Content.Role == "model" {
				for _, part := range event.Content.Parts {
					if part.Text != "" {
						result.WriteString(part.Text)
					}
				}
			}
		}
	}

	return result.String()
}
