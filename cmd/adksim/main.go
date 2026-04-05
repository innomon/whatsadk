package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/innomon/whatsadk/internal/adksim"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	appName := flag.String("app", "whatsadk", "App name for ADK simulation")
	flag.Parse()

	// Channel for communication between HTTP server and TUI
	requests := make(chan *adksim.IncomingRequest, 100)

	server := adksim.NewServer(*port, *appName, requests)
	
	// Run server in background
	go func() {
		if err := server.Run(); err != nil {
			fmt.Printf("Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Run TUI in foreground
	p := tea.NewProgram(adksim.NewModel(requests))
	if _, err := p.Run(); err != nil {
		fmt.Printf("TUI error: %v\n", err)
		os.Exit(1)
	}
}
