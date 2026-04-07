package main

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log"
	"os"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

var whitelist []string

func loadWhitelist() {
	data, err := os.ReadFile("whitelist.json")
	if err != nil {
		log.Printf("Warning: failed to read whitelist.json: %v. All users will be ignored.", err)
		return
	}
	if err := json.Unmarshal(data, &whitelist); err != nil {
		log.Printf("Error: failed to parse whitelist.json: %v. All users will be ignored.", err)
	}
}

func isWhitelisted(userID string) bool {
	for _, u := range whitelist {
		if u == userID {
			return true
		}
	}
	return false
}

func main() {
	ctx := context.Background()

	loadWhitelist()

	// Create a deterministic "Ignore" agent.
	// This agent only responds to whitelisted users.
	ignoreAgent, err := agent.New(agent.Config{
		Name:        "IgnoreAgent",
		Description: "An agent that only responds to whitelisted users and silently ignores others.",
		Run:         ignoreRun,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Setup launcher config
	cfg := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(ignoreAgent),
	}

	fmt.Println("🚀 Ignore Agent is ready.")
	fmt.Printf("Whitelisted users: %v\n", whitelist)
	fmt.Println("To run as API server, use: go run main.go web api")
	fmt.Println("To run as interactive console, use: go run main.go console")

	l := full.NewLauncher()
	if err := l.Execute(ctx, cfg, os.Args[1:]); err != nil {
		log.Fatalf("Launcher failed: %v", err)
	}
}

func ignoreRun(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		userID := invCtx.Session().UserID()

		if !isWhitelisted(userID) {
			fmt.Printf("🔇 Ignoring user %s (not in whitelist)\n", userID)
			// Create a new event for the silent ignore response
			event := session.NewEvent(invCtx.InvocationID())
			event.LLMResponse = model.LLMResponse{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{
							InlineData: &genai.Blob{
								MIMEType: "application/x-adk-silent-ignore",
								Data:     []byte("User not in whitelist"),
							},
						},
					},
				},
			}
			yield(event, nil)
			return
		}

		userInput := ""
		var mediaDetected []string

		if invCtx.UserContent() != nil {
			for _, part := range invCtx.UserContent().Parts {
				if part.Text != "" {
					userInput = strings.ToLower(strings.TrimSpace(part.Text))
				}
				if part.InlineData != nil {
					mediaDetected = append(mediaDetected, fmt.Sprintf("%s (%d bytes)", part.InlineData.MIMEType, len(part.InlineData.Data)))
				}
			}
		}

		var responseText string
		if len(mediaDetected) > 0 {
			responseText = fmt.Sprintf("🎨 I received the following multimedia parts:\n- %s\n", strings.Join(mediaDetected, "\n- "))
			if userInput != "" {
				responseText += fmt.Sprintf("\nAnd the text: '%s'", userInput)
			}
		} else if userInput == "hello" || userInput == "hi" {
			responseText = "Hello! You are on the whitelist. I am the Ignore Agent. I can help you with the following tasks:\n" +
				"1. 👋 Greet you (say 'hello')\n" +
				"2. 📋 List my capabilities\n" +
				"3. 🤖 Demonstrate silent ignore for non-whitelisted users"
		} else {
			responseText = fmt.Sprintf("I received: '%s'. Try saying 'hello' to see what I can do!", userInput)
		}

		// Create a new event for the response
		event := session.NewEvent(invCtx.InvocationID())
		event.LLMResponse = model.LLMResponse{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					{Text: responseText},
				},
			},
		}

		yield(event, nil)
	}
}
