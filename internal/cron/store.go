package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/innomon/whatsadk/internal/store"
)

type Store struct {
	s *store.Store
}

func NewStore(s *store.Store) *Store {
	return &Store{s: s}
}

func (s *Store) SaveSummary(ctx context.Context, jobName, summary string) error {
	path := fmt.Sprintf("cron/%s/summary", jobName)
	metadata := map[string]interface{}{
		"job_name": jobName,
		"updated":  time.Now().UTC(),
		"mime_type": "text/plain",
	}
	return s.s.PutFile(ctx, path, metadata, []byte(summary), time.Now().UTC())
}

func (s *Store) GetSummary(ctx context.Context, jobName string) (string, error) {
	path := fmt.Sprintf("cron/%s/summary", jobName)
	file, err := s.s.GetFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to get summary for %s: %w", jobName, err)
	}
	if file == nil {
		return "", nil
	}
	return string(file.Content), nil
}
