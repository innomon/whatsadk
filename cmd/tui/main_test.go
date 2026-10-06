package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/tui"
)

func TestDiscoverDatabasePath(t *testing.T) {
	// Scenario 1: Specified configured path exists
	tmpDir, err := os.MkdirTemp("", "whatsadk_tui_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbFile := filepath.Join(tmpDir, "custom.db")
	if err := os.WriteFile(dbFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write test db file: %v", err)
	}

	got := DiscoverDatabasePath(dbFile)
	if got != dbFile {
		t.Errorf("expected %s, got %s", dbFile, got)
	}

	// Scenario 2: Specified configured path missing, fallback returns configured
	missingPath := filepath.Join(tmpDir, "nonexistent.db")
	gotMissing := DiscoverDatabasePath(missingPath)
	if gotMissing != missingPath {
		t.Errorf("expected %s for missing path when no fallback present, got %s", missingPath, gotMissing)
	}
}

func TestAppModelStatusUpdate(t *testing.T) {
	cfg := &config.Config{
		P2P: config.P2PConfig{
			Enabled:    true,
			NodeID:     "test-node",
			SwarmTopic: "test-topic",
		},
	}

	model := tui.NewAppModel(cfg, nil)
	model.SetActiveDB("data/p2p_num1.db")
	model.SetP2PState("Direct WAL Fallback (P2P Locked)")

	view := model.View()
	if view == "" {
		t.Errorf("expected non-empty model view")
	}
}
