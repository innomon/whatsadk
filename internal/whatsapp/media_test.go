package whatsapp

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"bytes"
	"testing"
)

func TestProcessor_ProcessImage(t *testing.T) {
	p := NewProcessor()
	ctx := context.Background()

	// Create a small red image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}

	part, err := p.ProcessImage(ctx, buf.Bytes())
	if err != nil {
		t.Fatalf("ProcessImage failed: %v", err)
	}

	if part.InlineData.MimeType != "image/jpeg" {
		t.Errorf("Expected mimeType image/jpeg, got %s", part.InlineData.MimeType)
	}
	if len(part.InlineData.Data) == 0 {
		t.Error("Expected non-empty base64 data")
	}
}

func TestProcessor_ProcessDocument(t *testing.T) {
	p := NewProcessor()
	ctx := context.Background()

	tests := []struct {
		name     string
		data     []byte
		mime     string
		wantErr  bool
		wantMime string
	}{
		{
			name:     "PDF pass-through",
			data:     []byte("%PDF-1.4 test content"),
			mime:     "application/pdf",
			wantErr:  false,
			wantMime: "application/pdf",
		},
		{
			name:     "TXT pass-through",
			data:     []byte("hello world"),
			mime:     "text/plain",
			wantErr:  false,
			wantMime: "text/plain",
		},
		{
			name:    "Unsupported type",
			data:    []byte("binary data"),
			mime:    "application/x-executable",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part, err := p.ProcessDocument(ctx, tt.data, tt.mime)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessDocument() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if part.InlineData.MimeType != tt.wantMime {
					t.Errorf("Expected mime %s, got %s", tt.wantMime, part.InlineData.MimeType)
				}
			}
		})
	}
}
