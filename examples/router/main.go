package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	agenticconfig "github.com/innomon/agentic/pkg/config"
	"github.com/innomon/agentic/pkg/registry"
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
	"gopkg.in/yaml.v3"
)

type RouterConfig struct {
	DefaultApp string           `yaml:"default_app"`
	PgsqlURL   string           `yaml:"pgsql_url"`
	Apps       []AppConfig      `yaml:"apps"`
	Classifier ClassifierConfig `yaml:"classifier"`
	Prompts    PromptsConfig    `yaml:"prompts"`
}

type AppConfig struct {
	AppName       string `yaml:"appName"`
	A2AURL        string `yaml:"a2aURL"`
	ADKAppName    string `yaml:"adkAppName"`
	AgenticConfig string `yaml:"agenticConfig"`
	Title         string `yaml:"title"`
}

type RunnerManager struct {
	runners map[string]*runner.Runner
}

func NewRunnerManager() *RunnerManager {
	return &RunnerManager{
		runners: make(map[string]*runner.Runner),
	}
}

func (rm *RunnerManager) Init(ctx context.Context, apps []AppConfig) error {
	for _, app := range apps {
		if app.AgenticConfig == "" {
			continue
		}

		fmt.Printf("📦 Loading in-process agent for %s from %s\n", app.AppName, app.AgenticConfig)
		agenticCfg, err := agenticconfig.Load(app.AgenticConfig)
		if err != nil {
			log.Printf("Warning: failed to load agentic config for %s: %v", app.AppName, err)
			continue
		}

		reg := registry.New(agenticCfg)
		lc, err := reg.BuildLauncherConfig(ctx)
		if err != nil {
			log.Printf("Warning: failed to build launcher config for %s: %v", app.AppName, err)
			continue
		}

		ag, err := registry.Get[adkagent.Agent](ctx, reg, app.ADKAppName)
		if err != nil {
			log.Printf("Warning: failed to load agent %s: %v", app.ADKAppName, err)
			continue
		}

		r, err := runner.New(runner.Config{
			AppName:        app.ADKAppName,
			Agent:          ag,
			SessionService: lc.SessionService,
		})
		if err != nil {
			log.Printf("Warning: failed to create runner for agent %s: %v", app.ADKAppName, err)
			continue
		}
		rm.runners[app.AppName] = r
		fmt.Printf("✅ Registered local agent: %s\n", app.AppName)
	}
	return nil
}

func (rm *RunnerManager) Get(appName string) (*runner.Runner, bool) {
	r, ok := rm.runners[appName]
	return r, ok
}

type ClassifierConfig struct {
	Provider        string `yaml:"provider"`
	Model           string `yaml:"model"`
	Endpoint        string `yaml:"endpoint"`
	APIKey          string `yaml:"api_key"`
	Prompt          string `yaml:"prompt"`
	FallbackMessage string `yaml:"fallback_message"`
}

type PromptsConfig struct {
	Selection string `yaml:"selection"`
}

var (
	cfg           RouterConfig
	dbStore       *store.Store
	runnerManager *RunnerManager
)

func loadConfig() {
	configPath := "router.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if exe, err := os.Executable(); err == nil {
			exeConfig := filepath.Join(filepath.Dir(exe), "router.yaml")
			if _, err := os.Stat(exeConfig); err == nil {
				configPath = exeConfig
			}
		}
	}

	fmt.Printf("📖 Loading router config from %s\n", configPath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("failed to read %s: %v", configPath, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("failed to parse %s: %v", configPath, err)
	}
}

func main() {
	ctx := context.Background()
	loadConfig()

	// Adjust args for launcher if we consumed one for config
	launcherArgs := os.Args[1:]
	if len(os.Args) > 1 {
		launcherArgs = os.Args[2:]
	}

	var err error
	dbStore, err = store.Open(cfg.PgsqlURL)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer dbStore.Close()

	runnerManager = NewRunnerManager()
	if err := runnerManager.Init(ctx, cfg.Apps); err != nil {
		log.Fatalf("failed to initialize runner manager: %v", err)
	}

	routerAgent, err := adkagent.New(adkagent.Config{
		Name:        "RouterAgent",
		Description: "An agent that routes requests to other agents based on user configuration and state.",
		Run:         routerRun,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	launcherCfg := &launcher.Config{
		AgentLoader: adkagent.NewSingleLoader(routerAgent),
	}

	fmt.Println("🚀 Router Agent is ready.")
	l := full.NewLauncher()
	if err := l.Execute(ctx, launcherCfg, launcherArgs); err != nil {
		log.Fatalf("Launcher failed: %v", err)
	}
}

func routerRun(invCtx adkagent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		ctx := context.Background()
		userID := invCtx.Session().UserID()

		// 4. Determine isFromMe
		logs, err := dbStore.GetFilesysLogs(ctx, userID, 1)
		isFromMe := false
		if err == nil && len(logs) > 0 {
			var meta struct {
				Metadata struct {
					IsFromMe bool `json:"is_from_me"`
				} `json:"metadata"`
			}
			if logs[0].Metadata.Valid {
				if err := json.Unmarshal([]byte(logs[0].Metadata.String), &meta); err == nil {
					isFromMe = meta.Metadata.IsFromMe
				}
			}
		}

		// 5. Read router/<userID>/apps.json
		appsFile, err := dbStore.GetFile(ctx, "router/"+userID+"/apps.json")
		var userApps []string
		if err == nil && appsFile != nil {
			json.Unmarshal(appsFile.Content, &userApps)
		}

		if isFromMe {
			userApps = append(userApps, "admin", "su")
		}

		// Deduplicate
		userApps = deduplicate(userApps)

		// 6. Handle empty app list
		if len(userApps) == 0 {
			userApps = []string{cfg.DefaultApp}
		}

		targetApp := ""
		if len(userApps) == 1 {
			targetApp = userApps[0]
		} else {
			// 8. Handle multiple apps
			stateFile, err := dbStore.GetFile(ctx, "router/"+userID+"/state.json")
			var state struct {
				Status  string   `json:"status"`
				Options []string `json:"options"`
				App     string   `json:"app"`
			}
			if err == nil && stateFile != nil {
				json.Unmarshal(stateFile.Content, &state)
			}

			if state.Status == "" {
				state.Status = "pending_selection"
				state.Options = userApps
				stateJSON, _ := json.Marshal(state)
				dbStore.PutFile(ctx, "router/"+userID+"/state.json", nil, stateJSON, time.Now())

				optionsText := formatOptions(state.Options)
				responseText := strings.ReplaceAll(cfg.Prompts.Selection, "${optios}", optionsText)
				yield(makeResponse(invCtx, responseText), nil)
				return
			}

			if state.Status == "pending_selection" {
				userInput := getUserInput(invCtx)
				appIndex := -1

				// Match by exact number
				if idx, err := strconv.Atoi(userInput); err == nil && idx > 0 && idx <= len(state.Options) {
					appIndex = idx - 1
				} else {
					// Match by exact title
					for i, appName := range state.Options {
						if app := findApp(appName); app != nil && strings.EqualFold(app.Title, userInput) {
							appIndex = i
							break
						}
					}
				}

				// 9. Use OpenAI classifier if no match
				if appIndex == -1 {
					appIndex = classifyInput(ctx, userInput, state.Options)
				}

				if appIndex >= 0 {
					targetApp = state.Options[appIndex]
					// Clear state or update to routed
					dbStore.DeleteFile(ctx, "router/"+userID+"/state.json")
				} else {
					yield(makeResponse(invCtx, cfg.Classifier.FallbackMessage), nil)
					return
				}
			}
		}

		// 6. & 7. Route to target app
		if targetApp == "ignore" {
			event := session.NewEvent(invCtx.InvocationID())
			event.LLMResponse = model.LLMResponse{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{
							InlineData: &genai.Blob{
								MIMEType: "application/x-adk-silent-ignore",
								Data:     []byte("Ignored"),
							},
						},
					},
				},
			}
			yield(event, nil)
			return
		}

		app := findApp(targetApp)
		if app == nil {
			yield(makeResponse(invCtx, "Target application not found: "+targetApp), nil)
			return
		}

		if rnr, ok := runnerManager.Get(targetApp); ok {
			// Re-use session ID from router session
			sessionID := invCtx.Session().ID()
			
			// Prepare message for the in-process runner
			msg := &genai.Content{
				Role:  "user",
				Parts: invCtx.UserContent().Parts,
			}

			// Execute in-process
			events := rnr.Run(ctx, userID, sessionID, msg, adkagent.RunConfig{})
			for ev, err := range events {
				if err != nil {
					yield(nil, err)
					return
				}
				// We need to ensure InvocationID matches if needed, 
				// but session.Event already has its own ID.
				if !yield(ev, nil) {
					return
				}
			}
			return
		}

		appClient := agent.NewClient(&config.ADKConfig{Endpoint: app.A2AURL, AppName: app.ADKAppName}, nil)
		parts := genaiToAgentParts(invCtx.UserContent().Parts)
		respParts, err := appClient.ChatParts(ctx, userID, parts)
		if err != nil {
			yield(makeResponse(invCtx, "Error calling agent: "+err.Error()), nil)
			return
		}

		event := session.NewEvent(invCtx.InvocationID())
		event.LLMResponse = model.LLMResponse{
			Content: &genai.Content{
				Role:  "model",
				Parts: agentToGenaiParts(respParts),
			},
		}
		yield(event, nil)
	}
}

func deduplicate(ss []string) []string {
	m := make(map[string]bool)
	var res []string
	for _, s := range ss {
		if !m[s] {
			m[s] = true
			res = append(res, s)
		}
	}
	return res
}

func formatOptions(options []string) string {
	var lines []string
	for i, appName := range options {
		title := appName
		if app := findApp(appName); app != nil {
			title = app.Title
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, title))
	}
	return strings.Join(lines, "\n")
}

func findApp(name string) *AppConfig {
	for _, app := range cfg.Apps {
		if app.AppName == name {
			return &app
		}
	}
	return nil
}

func getUserInput(invCtx adkagent.InvocationContext) string {
	if invCtx.UserContent() == nil {
		return ""
	}
	for _, p := range invCtx.UserContent().Parts {
		if p.Text != "" {
			return strings.TrimSpace(p.Text)
		}
	}
	return ""
}

func makeResponse(invCtx adkagent.InvocationContext, text string) *session.Event {
	event := session.NewEvent(invCtx.InvocationID())
	event.LLMResponse = model.LLMResponse{
		Content: &genai.Content{
			Role: "model",
			Parts: []*genai.Part{
				{Text: text},
			},
		},
	}
	return event
}

func genaiToAgentParts(parts []*genai.Part) []agent.Part {
	var res []agent.Part
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Text != "" {
			res = append(res, agent.Part{Text: p.Text})
		} else if p.InlineData != nil {
			res = append(res, agent.Part{
				InlineData: &agent.InlineData{
					MimeType: p.InlineData.MIMEType,
					Data:     base64.StdEncoding.EncodeToString(p.InlineData.Data),
				},
			})
		}
	}
	return res
}

func agentToGenaiParts(parts []agent.Part) []*genai.Part {
	var res []*genai.Part
	for _, p := range parts {
		if p.Text != "" {
			res = append(res, &genai.Part{Text: p.Text})
		} else if p.InlineData != nil {
			data, _ := base64.StdEncoding.DecodeString(p.InlineData.Data)
			res = append(res, &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: p.InlineData.MimeType,
					Data:     data,
				},
			})
		}
	}
	return res
}

func classifyInput(ctx context.Context, text string, options []string) int {
	if cfg.Classifier.APIKey == "" {
		return -1
	}

	var optionTitles []string
	for _, appName := range options {
		title := appName
		if app := findApp(appName); app != nil {
			title = app.Title
		}
		optionTitles = append(optionTitles, title)
	}

	prompt := strings.ReplaceAll(cfg.Classifier.Prompt, "${text}", text)
	prompt = strings.ReplaceAll(prompt, "${optios}", strings.Join(optionTitles, ", "))

	reqBody, _ := json.Marshal(map[string]any{
		"model": cfg.Classifier.Model,
		"messages": []any{
			map[string]string{"role": "user", "content": prompt},
		},
		"temperature": 0,
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", cfg.Classifier.Endpoint+"/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Classifier.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Choices) == 0 {
		return -1
	}

	content := strings.TrimSpace(res.Choices[0].Message.Content)
	idx, err := strconv.Atoi(content)
	if err != nil || idx <= 0 || idx > len(options) {
		return -1
	}

	return idx - 1
}
eturn idx - 1
}
