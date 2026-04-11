package cron

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/robfig/cron/v3"
)

type Manager struct {
	cron   *cron.Cron
	cfg    *config.Config
	store  *Store
	jwtGen *auth.JWTGenerator
	ctx    context.Context
}

func NewManager(ctx context.Context, cfg *config.Config, store *Store, jwtGen *auth.JWTGenerator) *Manager {
	return &Manager{
		cron:   cron.New(cron.WithSeconds()), // WithSeconds for more precision if needed
		cfg:    cfg,
		store:  store,
		jwtGen: jwtGen,
		ctx:    ctx,
	}
}

func (m *Manager) Start() error {
	if !m.cfg.Cron.Enabled {
		return nil
	}

	for _, jobCfg := range m.cfg.Cron.Jobs {
		job := jobCfg // copy for closure
		_, err := m.cron.AddFunc(job.Schedule, func() {
			m.runJob(job)
		})
		if err != nil {
			return fmt.Errorf("failed to add job %s: %w", job.Name, err)
		}
		log.Printf("[Cron] Scheduled job: %s with schedule: %s", job.Name, job.Schedule)
	}

	m.cron.Start()
	return nil
}

func (m *Manager) Stop() {
	m.cron.Stop()
}

func (m *Manager) runJob(job config.CronJobConfig) {
	log.Printf("[Cron] Running job: %s", job.Name)
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
	defer cancel()

	// 1. Get previous summary
	var prevSummary string
	var err error
	if m.store != nil {
		prevSummary, err = m.store.GetSummary(ctx, job.Name)
		if err != nil {
			log.Printf("[Cron] Error getting summary for job %s: %v", job.Name, err)
		}
	}

	// 2. Prepare message
	fullMessage := job.Message
	if prevSummary != "" {
		fullMessage = fmt.Sprintf("Previous Run Summary:\n%s\n\nTask:\n%s", prevSummary, job.Message)
	}

	// 3. Initialize Agent Client
	// Use job specific agent config if provided, otherwise fallback to global
	agentCfg := job.Agent
	if agentCfg.Endpoint == "" {
		agentCfg = m.cfg.ADK
	}

	if agentCfg.Endpoint == "" {
		log.Printf("[Cron] Job %s skipped: no agent endpoint configured", job.Name)
		return
	}
	
	client := agent.NewClient(&agentCfg, m.jwtGen)

	// 4. Run Agent
	parts, err := client.Chat(ctx, job.UserID, fullMessage)
	if err != nil {
		log.Printf("[Cron] Job %s failed: %v", job.Name, err)
		return
	}

	// 5. Extract response text as new summary
	var newSummary string
	for _, p := range parts {
		if p.Text != "" {
			newSummary += p.Text
		}
	}

	if newSummary == "" {
		log.Printf("[Cron] Job %s returned empty summary", job.Name)
		return
	}

	// 6. Save new summary
	if m.store != nil {
		if err := m.store.SaveSummary(ctx, job.Name, newSummary); err != nil {
			log.Printf("[Cron] Error saving summary for job %s: %v", job.Name, err)
		}
	}

	log.Printf("[Cron] Job %s completed successfully", job.Name)
}
