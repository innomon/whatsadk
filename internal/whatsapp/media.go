package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/nfnt/resize"
)

const (
	MaxMediaSize    = 20 * 1024 * 1024 // 20MB
	TargetImageSize = 896
	MaxVideoFrames  = 20
	ProcessTimeout  = 60 * time.Second
)

// Processor handles media transformation for ADK.
type Processor struct {
	tempDir string
}

func NewProcessor() *Processor {
	tempDir := filepath.Join(os.TempDir(), "whatsadk-media")
	os.MkdirAll(tempDir, 0755)
	return &Processor{tempDir: tempDir}
}

func (p *Processor) ProcessImage(ctx context.Context, data []byte) (*agent.Part, error) {
	if len(data) > MaxMediaSize {
		return nil, fmt.Errorf("image too large: %d bytes", len(data))
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width, height := uint(bounds.Dx()), uint(bounds.Dy())

	// Resize if too large or for normalization
	if width > 2000 || height > 2000 || len(data) > 1500000 {
		if width < height {
			img = resize.Resize(TargetImageSize, 0, img, resize.Lanczos3)
		} else {
			img = resize.Resize(0, TargetImageSize, img, resize.Lanczos3)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("failed to encode jpeg: %w", err)
	}

	return &agent.Part{
		InlineData: &agent.InlineData{
			MimeType: "image/jpeg",
			Data:     base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	}, nil
}

func (p *Processor) ProcessAudio(ctx context.Context, data []byte) (*agent.Part, error) {
	if len(data) > MaxMediaSize {
		return nil, fmt.Errorf("audio too large: %d bytes", len(data))
	}

	// Use ffmpeg to convert to WAV (PCM 16-bit, 16kHz, mono)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", "pipe:0",
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		"-f", "wav",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(data)
	var out bytes.Buffer
	cmd.Stderr = os.Stderr // For debugging
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg audio conversion failed: %w", err)
	}

	return &agent.Part{
		InlineData: &agent.InlineData{
			MimeType: "audio/wav",
			Data:     base64.StdEncoding.EncodeToString(out.Bytes()),
		},
	}, nil
}

func (p *Processor) ProcessVideo(ctx context.Context, data []byte) ([]agent.Part, error) {
	if len(data) > MaxMediaSize {
		return nil, fmt.Errorf("video too large: %d bytes", len(data))
	}

	// Create temp file for video
	tmpFile, err := os.CreateTemp(p.tempDir, "video-*.mp4")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write video to temp file: %w", err)
	}
	tmpFile.Close()

	// Output directory for frames
	outDir, err := os.MkdirTemp(p.tempDir, "frames-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create frames dir: %w", err)
	}
	defer os.RemoveAll(outDir)

	// Sample frames at 1 FPS
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", tmpFile.Name(),
		"-vf", "fps=1,scale='if(gt(iw,ih),-1,896)':'if(gt(iw,ih),896,-1)'",
		"-vframes", fmt.Sprintf("%d", MaxVideoFrames),
		"-q:v", "2",
		filepath.Join(outDir, "frame-%03d.jpg"),
	)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg video sampling failed: %w", err)
	}

	files, err := os.ReadDir(outDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read frames dir: %w", err)
	}

	var parts []agent.Part
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".jpg") {
			continue
		}
		frameData, err := os.ReadFile(filepath.Join(outDir, file.Name()))
		if err != nil {
			continue
		}
		parts = append(parts, agent.Part{
			InlineData: &agent.InlineData{
				MimeType: "image/jpeg",
				Data:     base64.StdEncoding.EncodeToString(frameData),
			},
		})
	}

	return parts, nil
}

func (p *Processor) ProcessDocument(ctx context.Context, data []byte, mimeType string) (*agent.Part, error) {
	if len(data) > MaxMediaSize {
		return nil, fmt.Errorf("document too large: %d bytes", len(data))
	}

	// For now, we only support passing through PDF, TXT, CSV
	allowedMimes := map[string]bool{
		"application/pdf": true,
		"text/plain":      true,
		"text/csv":        true,
	}

	if !allowedMimes[mimeType] {
		// Attempt to detect if it's text even if mimeType is generic
		detected := http.DetectContentType(data)
		if !allowedMimes[detected] {
			return nil, fmt.Errorf("unsupported document type: %s", mimeType)
		}
		mimeType = detected
	}

	return &agent.Part{
		InlineData: &agent.InlineData{
			MimeType: mimeType,
			Data:     base64.StdEncoding.EncodeToString(data),
		},
	}, nil
}
