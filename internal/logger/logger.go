package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/config"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// MultiHandler multiplexes logs to multiple handlers
type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: next}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: next}
}

// Global Logger reference
var Log *slog.Logger = slog.Default()

// Init initializes the global logger based on Config
func Init(cfg *config.Config) (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToUpper(cfg.Logging.Level) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN", "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	var handlers []slog.Handler

	if cfg.Logging.ConsoleEnabled {
		// Console gets human-readable Text logs
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}))
	}

	if cfg.Logging.FileEnabled {
		rotator, err := NewLogRotator(cfg.Logging.Dir, cfg.Logging.FileName, cfg.Logging.MaxSizeMB, cfg.Logging.MaxBackups)
		if err != nil {
			return nil, fmt.Errorf("failed to init log rotator: %w", err)
		}
		// JSONL logs to file including source info
		handlers = append(handlers, slog.NewJSONHandler(rotator, opts))
	}

	if len(handlers) == 0 {
		// Fallback to no-op if all disabled
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}

	Log = slog.New(NewMultiHandler(handlers...))
	slog.SetDefault(Log)

	return Log, nil
}

// WhatsMeowLogger wraps slog.Logger to implement go.mau.fi/whatsmeow/util/log.Logger
type WhatsMeowLogger struct {
	logger *slog.Logger
	module string
}

func NewWhatsMeowLogger(s *slog.Logger, module string) waLog.Logger {
	if s == nil {
		s = Log
	}
	if module != "" {
		s = s.With("module", module)
	}
	return &WhatsMeowLogger{
		logger: s,
		module: module,
	}
}

func (w *WhatsMeowLogger) log(level slog.Level, format string, args ...interface{}) {
	ctx := context.Background()
	if !w.logger.Handler().Enabled(ctx, level) {
		return
	}
	msg := fmt.Sprintf(format, args...)

	// Retrieve calling frame to pass original PC to slog
	var pcs [3]uintptr
	n := runtime.Callers(3, pcs[:])
	var pc uintptr
	if n > 0 {
		frames := runtime.CallersFrames(pcs[:n])
		for {
			frame, more := frames.Next()
			// Skip internal log helper methods to get real caller
			if !strings.Contains(frame.Function, "WhatsMeowLogger") && !strings.Contains(frame.Function, "runtime.") {
				pc = frame.PC
				break
			}
			if !more {
				break
			}
		}
	}

	r := slog.NewRecord(time.Now(), level, msg, pc)
	_ = w.logger.Handler().Handle(ctx, r)
}

func (w *WhatsMeowLogger) Debugf(format string, args ...interface{}) {
	w.log(slog.LevelDebug, format, args...)
}

func (w *WhatsMeowLogger) Infof(format string, args ...interface{}) {
	w.log(slog.LevelInfo, format, args...)
}

func (w *WhatsMeowLogger) Warnf(format string, args ...interface{}) {
	w.log(slog.LevelWarn, format, args...)
}

func (w *WhatsMeowLogger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Demote common websocket EOF errors which are automatically handled by reconnections.
	if strings.Contains(msg, "Error reading from websocket") && (strings.Contains(msg, "EOF") || strings.Contains(msg, "connection reset by peer")) {
		w.log(slog.LevelInfo, "Websocket connection lost (%s), whatsmeow will reconnect automatically.", msg)
		return
	}
	w.log(slog.LevelError, format, args...)
}

func (w *WhatsMeowLogger) Sub(module string) waLog.Logger {
	newModule := module
	if w.module != "" {
		newModule = w.module + "/" + module
	}
	return NewWhatsMeowLogger(w.logger, newModule)
}
