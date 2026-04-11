package cron

import (
	"context"
	"testing"
	"time"

	"github.com/innomon/whatsadk/internal/config"
)

func TestCronScheduling(t *testing.T) {
	cfg := &config.Config{
		Cron: config.CronConfig{
			Enabled: true,
			Jobs: []config.CronJobConfig{
				{
					Name:     "test-job",
					Schedule: "@every 1s",
					UserID:   "test-user",
					Message:  "test message",
				},
			},
		},
	}

	// We can't easily test the actual run without mocking the agent client and store,
	// but we can test if the manager starts without error.
	mgr := NewManager(context.Background(), cfg, nil, nil)
	err := mgr.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer mgr.Stop()

	// Wait a bit to ensure no panics
	time.Sleep(2 * time.Second)
}
