package agent

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/innomon/whatsadk/internal/config"
)

type Gateway struct {
	runner         *runner.Runner
	sessionService session.Service
	appName        string
}

func New(ctx context.Context, cfg *config.AgentConfig) (*Gateway, error) {
	model, err := gemini.NewModel(ctx, cfg.Model, &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini model: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        cfg.Name,
		Model:       model,
		Description: cfg.Description,
		Instruction: cfg.Instruction,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	sessionService := session.InMemoryService()

	r, err := runner.New(runner.Config{
		AppName:        cfg.Name,
		Agent:          a,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	return &Gateway{
		runner:         r,
		sessionService: sessionService,
		appName:        cfg.Name,
	}, nil
}

func (g *Gateway) Chat(ctx context.Context, userID, message string) (string, error) {
	sessionID := userID

	_, err := g.sessionService.Get(ctx, &session.GetRequest{
		AppName:   g.appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		_, err = g.sessionService.Create(ctx, &session.CreateRequest{
			AppName:   g.appName,
			UserID:    userID,
			SessionID: sessionID,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create session: %w", err)
		}
	}

	userContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			genai.NewPartFromText(message),
		},
	}

	var responseText strings.Builder
	for event, err := range g.runner.Run(ctx, userID, sessionID, userContent, agent.RunConfig{}) {
		if err != nil {
			return "", fmt.Errorf("agent run error: %w", err)
		}

		if event.Content != nil && event.IsFinalResponse() {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					responseText.WriteString(part.Text)
				}
			}
		}
	}

	return responseText.String(), nil
}
