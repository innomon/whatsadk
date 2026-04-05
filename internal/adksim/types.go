package adksim

import (
	"github.com/innomon/whatsadk/internal/agent"
)

// IncomingRequest represents a prompt received from the gateway.
type IncomingRequest struct {
	ID        string           `json:"id"`
	AppName   string           `json:"appName"`
	UserID    string           `json:"userId"`
	SessionID string           `json:"sessionId"`
	NewMessage *agent.Message  `json:"newMessage"`
	Streaming bool             `json:"streaming"`
	
	// ResponseChan is used by the TUI to send the human's response back to the HTTP handler.
	ResponseChan chan OutgoingResponse
}

// OutgoingResponse represents the human-provided response from the TUI.
type OutgoingResponse struct {
	Parts []agent.Part
	Err   error
}
