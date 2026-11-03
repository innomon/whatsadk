package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/tui"
	"gopkg.in/yaml.v3"
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

func TestStagingConfigTUI(t *testing.T) {
	path := "/home/innomon/staging/whatsadk/config_tui.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal YAML from %s: %v", path, err)
	}

	if !cfg.P2P.Enabled {
		t.Errorf("expected P2P.Enabled to be true")
	}
	if cfg.P2P.NodeID != "tui-node-1" {
		t.Errorf("expected NodeID 'tui-node-1', got %q", cfg.P2P.NodeID)
	}
	if cfg.P2P.SwarmTopic != "whatsadk-mesh-topic" {
		t.Errorf("expected SwarmTopic 'whatsadk-mesh-topic', got %q", cfg.P2P.SwarmTopic)
	}
	if cfg.P2P.DBPath != "data/tui_p2p.db" {
		t.Errorf("expected DBPath 'data/tui_p2p.db', got %q", cfg.P2P.DBPath)
	}
	if len(cfg.Commands) != 1 {
		t.Fatalf("expected 1 command definition, got %d", len(cfg.Commands))
	}
	cmd := cfg.Commands[0]
	if cmd.Name != "sql" || cmd.Handler != "sql_exec" {
		t.Errorf("unexpected command definition: %+v", cmd)
	}
}
