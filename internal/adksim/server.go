package adksim

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/innomon/whatsadk/internal/agent"
)

type Server struct {
	port      int
	appName   string
	requests  chan *IncomingRequest
	mediaDir  string
}

func NewServer(port int, appName string, requests chan *IncomingRequest) *Server {
	mediaDir := "adk_media_received"
	_ = os.MkdirAll(mediaDir, 0755)
	return &Server{
		port:     port,
		appName:  appName,
		requests: requests,
		mediaDir: mediaDir,
	}
}

func (s *Server) Run() error {
	mux := http.NewServeMux()
	
	// Session management (always 200)
	mux.HandleFunc("POST /apps/{appName}/users/{userID}/sessions/{sessionID}", s.handleSession)
	
	// Chat endpoints
	mux.HandleFunc("POST /run", s.handleRun)
	mux.HandleFunc("POST /run_sse", s.handleRunSSE)

	fmt.Printf("🚀 ADK Reverse Simulator listening on :%d (App: %s)\n", s.port, s.appName)
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), mux)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var runReq agent.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&runReq); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	incoming := s.processRequest(&runReq, false)
	s.requests <- incoming

	// Block until TUI provides response
	resp := <-incoming.ResponseChan
	if resp.Err != nil {
		http.Error(w, resp.Err.Error(), http.StatusInternalServerError)
		return
	}

	// Wrap in Event (final state)
	events := []agent.Event{
		{
			Content: &agent.Content{
				Role:  "model",
				Parts: resp.Parts,
			},
			Partial: false,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (s *Server) handleRunSSE(w http.ResponseWriter, r *http.Request) {
	var runReq agent.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&runReq); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	incoming := s.processRequest(&runReq, true)
	s.requests <- incoming

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Wait for response parts from TUI
	// For simplicity in this mock, we wait for ONE consolidated response but send it as SSE data
	// To truly simulate streaming, we'd need a more granular response channel.
	// For now, let's just send the whole response as one "final" event.
	resp := <-incoming.ResponseChan
	if resp.Err != nil {
		fmt.Fprintf(w, "data: %s\n\n", resp.Err.Error())
		return
	}

	event := agent.Event{
		Content: &agent.Content{
			Role:  "model",
			Parts: resp.Parts,
		},
		Partial: false,
	}

	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) processRequest(req *agent.RunRequest, streaming bool) *IncomingRequest {
	id := uuid.New().String()
	
	// Extract media if present
	if req.NewMessage != nil {
		for i, part := range req.NewMessage.Parts {
			if part.InlineData != nil {
				path, err := s.saveMedia(part.InlineData)
				if err == nil {
					// Add a "virtual" text part to show path in TUI
					// We modify a clone to avoid side effects? 
					// Actually, the TUI will display the file path.
					fmt.Printf("[ADKSim] Saved media from %s: %s\n", req.UserID, path)
				}
			}
			_ = i
		}
	}

	return &IncomingRequest{
		ID:           id,
		AppName:      req.AppName,
		UserID:       req.UserID,
		SessionID:    req.SessionID,
		NewMessage:   req.NewMessage,
		Streaming:    streaming,
		ResponseChan: make(chan OutgoingResponse),
	}
}

func (s *Server) saveMedia(data *agent.InlineData) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(data.Data)
	if err != nil {
		return "", err
	}

	ext := mimeToExt(data.MimeType)
	filename := fmt.Sprintf("adk_%d%s", time.Now().UnixNano(), ext)
	path := filepath.Join(s.mediaDir, filename)

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return "", err
	}

	return path, nil
}

// mimeToExt copied from simulator.go for consistency (could be refactored to internal/media/utils.go later)
func mimeToExt(mime string) string {
	switch mime {
	case "image/jpeg": return ".jpg"
	case "image/png":  return ".png"
	case "image/webp": return ".webp"
	case "audio/wav":  return ".wav"
	case "audio/mpeg": return ".mp3"
	case "audio/ogg", "audio/opus": return ".ogg"
	case "video/mp4":  return ".mp4"
	case "application/pdf": return ".pdf"
	case "text/plain": return ".txt"
	case "text/csv":   return ".csv"
	default:           return ".bin"
	}
}
