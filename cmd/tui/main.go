// Package main implements the Interactive Command TUI and MCP UI shell for WhatsADK.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/tui"
)

// DiscoverDatabasePath checks if the configured database path exists.
// If missing or empty, it auto-discovers co-located gateway databases (e.g. data/p2p_num1.db, data/p2p_num2.db, whatsadk_p2p.db).
func DiscoverDatabasePath(configured string) string {
	if configured != "" {
		clean := configured
		for _, prefix := range []string{"sqlite-p2p://", "sqlite://", "p2p://", "pear://"} {
			if strings.HasPrefix(clean, prefix) {
				clean = strings.TrimPrefix(clean, prefix)
				break
			}
		}
		clean = strings.Split(clean, "?")[0]
		if _, err := os.Stat(clean); err == nil {
			return configured
		}
	}

	candidates := []string{
		"data/p2p_num1.db",
		"data/p2p_num2.db",
		"data/whatsadk_p2p.db",
		"p2p_num1.db",
		"p2p_num2.db",
		"whatsadk_p2p.db",
	}
	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}

	if matches, err := filepath.Glob("data/*.db"); err == nil && len(matches) > 0 {
		return matches[0]
	}
	if matches, err := filepath.Glob("*.db"); err == nil && len(matches) > 0 {
		return matches[0]
	}

	return configured
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: failed to load config file: %v. Running with defaults.", err)
		cfg = &config.Config{}
	}

	targetDB := DiscoverDatabasePath(cfg.P2P.DBPath)
	if targetDB == "" {
		targetDB = DiscoverDatabasePath(cfg.Verification.DatabaseURL)
	}
	if targetDB == "" {
		targetDB = DiscoverDatabasePath(cfg.WhatsApp.StoreDSN)
	}

	var s *store.Store
	var activeDB string
	var p2pState string

	if cfg.P2P.Enabled {
		nodeCfg := cfg.P2P.ToNodeConfig()
		if targetDB != "" {
			nodeCfg.DBPath = targetDB
		}
		var p2pErr error
		s, p2pErr = store.OpenP2PFromNodeConfig(nodeCfg)
		if p2pErr != nil {
			if targetDB != "" {
				var directErr error
				s, directErr = store.Open(targetDB)
				if directErr == nil && s != nil {
					activeDB = targetDB
					p2pState = "Direct WAL Fallback (P2P Locked)"
					log.Printf("TUI opened direct SQLite WAL store at %s (P2P lock active: %v)", targetDB, p2pErr)
				} else {
					p2pState = "Offline Mode"
					log.Printf("Notice: P2P store offline (%v, %v). TUI running in offline mode.", p2pErr, directErr)
				}
			} else {
				p2pState = "Offline Mode"
				log.Printf("Notice: P2P store offline (%v). TUI running in offline mode.", p2pErr)
			}
		} else {
			activeDB = nodeCfg.DBPath
			p2pState = fmt.Sprintf("P2P Swarm Node: %s (Topic: %s)", cfg.P2P.NodeID, cfg.P2P.SwarmTopic)
			log.Printf("TUI SQLite P2P mesh enabled (Node: %s, Topic: %s)", cfg.P2P.NodeID, cfg.P2P.SwarmTopic)
		}
	} else {
		if targetDB != "" {
			var openErr error
			s, openErr = store.Open(targetDB)
			if openErr != nil {
				p2pState = "Offline Mode"
				log.Printf("Notice: store open error (%v). TUI running in memory mode.", openErr)
			} else {
				activeDB = targetDB
				p2pState = "Local SQLite Store"
			}
		} else {
			p2pState = "Offline Mode"
		}
	}
	if s != nil {
		defer s.Close()
	}

	model := tui.NewAppModel(cfg, s)
	if activeDB != "" {
		model.SetActiveDB(activeDB)
	}
	if p2pState != "" {
		model.SetP2PState(p2pState)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI program: %v\n", err)
		os.Exit(1)
	}
}
