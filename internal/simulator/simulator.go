package simulator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/whatsapp"
)

type Message struct {
	Role      string       `json:"role"`
	Parts     []agent.Part `json:"parts"`
	Timestamp time.Time    `json:"timestamp"`
}

type Session struct {
	UserID   string    `json:"user_id"`
	Messages []Message `json:"messages"`
}

type Simulator struct {
	adkClient *agent.Client
	mediaProc *whatsapp.Processor
	userID    string
	history   []Message
	mediaDir  string
}

func New(adkClient *agent.Client, mediaProc *whatsapp.Processor) *Simulator {
	mediaDir := "media_received"
	_ = os.MkdirAll(mediaDir, 0755)
	return &Simulator{
		adkClient: adkClient,
		mediaProc: mediaProc,
		userID:    "1234567890", // default user
		mediaDir:  mediaDir,
	}
}

func (s *Simulator) SetUserID(id string) {
	s.userID = id
	s.history = nil // Clear history for new user
}

func (s *Simulator) UserID() string {
	return s.userID
}

func (s *Simulator) SendText(ctx context.Context, text string) (string, error) {
	msg := Message{
		Role:      "user",
		Parts:     []agent.Part{{Text: text}},
		Timestamp: time.Now(),
	}
	s.history = append(s.history, msg)

	resp, err := s.adkClient.Chat(ctx, s.userID, text)
	if err != nil {
		return "", err
	}

	respMsg := Message{
		Role:      "model",
		Parts:     resp,
		Timestamp: time.Now(),
	}
	s.history = append(s.history, respMsg)

	return s.formatParts(resp), nil
}

func (s *Simulator) SendWithAttachment(ctx context.Context, text string, filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var parts []agent.Part
	if text != "" {
		parts = append(parts, agent.Part{Text: text})
	}

	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		part, err := s.mediaProc.ProcessImage(ctx, data)
		if err != nil {
			return "", err
		}
		parts = append(parts, *part)
	case ".mp3", ".wav", ".ogg", ".opus":
		part, err := s.mediaProc.ProcessAudio(ctx, data)
		if err != nil {
			return "", err
		}
		parts = append(parts, *part)
	case ".mp4", ".mkv", ".avi":
		vParts, err := s.mediaProc.ProcessVideo(ctx, data)
		if err != nil {
			return "", err
		}
		parts = append(parts, vParts...)
	default:
		// Default to document
		mimeType := "application/octet-stream"
		if ext == ".pdf" {
			mimeType = "application/pdf"
		} else if ext == ".txt" {
			mimeType = "text/plain"
		} else if ext == ".csv" {
			mimeType = "text/csv"
		}
		part, err := s.mediaProc.ProcessDocument(ctx, data, mimeType)
		if err != nil {
			return "", err
		}
		parts = append(parts, *part)
	}

	msg := Message{
		Role:      "user",
		Parts:     parts,
		Timestamp: time.Now(),
	}
	s.history = append(s.history, msg)

	resp, err := s.adkClient.ChatParts(ctx, s.userID, parts)
	if err != nil {
		return "", err
	}

	respMsg := Message{
		Role:      "model",
		Parts:     resp,
		Timestamp: time.Now(),
	}
	s.history = append(s.history, respMsg)

	return s.formatParts(resp), nil
}

func (s *Simulator) ExportSession(path string) error {
	sess := Session{
		UserID:   s.userID,
		Messages: s.history,
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Simulator) ImportSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Simulator) Replay(ctx context.Context, sess *Session, onMsg func(Message)) error {
	s.userID = sess.UserID
	s.history = nil
	for _, m := range sess.Messages {
		if m.Role == "user" {
			onMsg(m)
			resp, err := s.adkClient.ChatParts(ctx, s.userID, m.Parts)
			if err != nil {
				return err
			}
			respMsg := Message{
				Role:      "model",
				Parts:     resp,
				Timestamp: time.Now(),
			}
			s.history = append(s.history, m)
			s.history = append(s.history, respMsg)
			onMsg(respMsg)
		}
	}
	return nil
}

func (s *Simulator) formatParts(parts []agent.Part) string {
	var sb strings.Builder
	for _, p := range parts {
		if p.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.Text)
		}
		if p.InlineData != nil {
			path, err := s.saveMedia(p.InlineData)
			if err != nil {
				sb.WriteString(fmt.Sprintf("\n[Attachment error: %v]", err))
			} else {
				sb.WriteString(fmt.Sprintf("\n[Attachment: %s]", path))
			}
		}
	}
	return sb.String()
}

func (s *Simulator) saveMedia(data *agent.InlineData) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(data.Data)
	if err != nil {
		return "", err
	}

	ext := mimeToExt(data.MimeType)
	filename := fmt.Sprintf("media_%d%s", time.Now().UnixNano(), ext)
	path := filepath.Join(s.mediaDir, filename)

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return "", err
	}

	return path, nil
}

func mimeToExt(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "audio/wav":
		return ".wav"
	case "audio/mpeg":
		return ".mp3"
	case "audio/ogg", "audio/opus":
		return ".ogg"
	case "video/mp4":
		return ".mp4"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	default:
		return ".bin"
	}
}
