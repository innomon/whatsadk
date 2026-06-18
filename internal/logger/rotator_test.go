package logger

import (
	"os"
	"testing"
)

func TestLogRotator(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "whatsadk_log_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filename := "test.log"
	rotator, err := NewLogRotator(tempDir, filename, 1, 2)
	if err != nil {
		t.Fatalf("failed to create rotator: %v", err)
	}
	defer rotator.Close()

	// Write 1.1 MB of data to trigger rotation (1.1 * 1024 * 1024 = 1,153,433 bytes)
	chunkSize := 1024
	chunk := make([]byte, chunkSize)
	for i := 0; i < chunkSize; i++ {
		chunk[i] = 'a'
	}

	// Write 1150 chunks of 1KB to exceed 1MB
	for i := 0; i < 1150; i++ {
		_, err := rotator.Write(chunk)
		if err != nil {
			t.Fatalf("failed to write chunk %d: %v", i, err)
		}
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	var foundBackup bool
	for _, f := range files {
		if f.Name() != filename {
			foundBackup = true
		}
	}

	if !foundBackup {
		t.Errorf("expected to find a backup log file, but none was created")
	}
}
