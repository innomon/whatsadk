// Package main implements the Interactive Command TUI and MCP UI shell for WhatsADK.
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: failed to load config file: %v. Running with defaults.", err)
		cfg = &config.Config{}
	}

	var s *store.Store
	if cfg.P2P.Enabled {
		s, err = store.OpenP2PFromNodeConfig(cfg.P2P.ToNodeConfig())
		if err != nil {
			dbPath := cfg.P2P.DBPath
			if dbPath == "" {
				dbPath = cfg.Verification.DatabaseURL
			}
			if dbPath == "" {
				dbPath = cfg.WhatsApp.StoreDSN
			}
			var directErr error
			if dbPath != "" {
				s, directErr = store.Open(dbPath)
			}
			if directErr != nil || s == nil {
				log.Printf("Notice: P2P store offline (%v, %v). TUI running in offline/memory mode.", err, directErr)
			} else {
				log.Printf("TUI running in direct local SQLite P2P mode")
			}
		} else {
			log.Printf("TUI SQLite P2P mesh enabled (Node: %s, Topic: %s)", cfg.P2P.NodeID, cfg.P2P.SwarmTopic)
		}
	} else {
		dbURL := cfg.Verification.DatabaseURL
		if dbURL == "" {
			dbURL = cfg.WhatsApp.StoreDSN
		}
		if dbURL != "" {
			s, err = store.Open(dbURL)
			if err != nil {
				log.Printf("Notice: store open error (%v). TUI running in memory mode.", err)
			}
		}
	}
	if s != nil {
		defer s.Close()
	}

	model := tui.NewAppModel(cfg, s)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI program: %v\n", err)
		os.Exit(1)
	}
}
