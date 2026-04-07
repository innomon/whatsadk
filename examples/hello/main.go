package main

import (
	"context"
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

func main() {
	ctx := context.Background()

	// Create a deterministic "Hello Work" agent.
	// This agent doesn't use an LLM. It responds to "hello" with a fixed message.
	helloAgent, err := agent.New(agent.Config{
		Name:        "HelloWorkAgent",
		Description: "A simple deterministic agent that says hello and lists its capabilities.",
		Run:         helloWorkRun,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Setup launcher config
	cfg := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(helloAgent),
	}

	fmt.Println("🚀 Hello Work Agent is ready.")
	fmt.Println("To run as API server, use: go run main.go web api")
	fmt.Println("To run as API server on custom port: go run main.go web --port <port> api")
	fmt.Println("To run as interactive console, use: go run main.go console")
	
	l := full.NewLauncher()
	if err := l.Execute(ctx, cfg, os.Args[1:]); err != nil {
		log.Fatalf("Launcher failed: %v", err)
	}
}

func helloWorkRun(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
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
			responseText = "Hello! I am the Hello Work Agent. I can help you with the following tasks:\n" +
				"1. 👋 Greet you (say 'hello')\n" +
				"2. 📋 List my capabilities\n" +
				"3. 🤖 Demonstrate a deterministic ADK agent integration"
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
