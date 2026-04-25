package whatsapp

import (
	"fmt"
	"strings"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// FilteredLogger wraps a waLog.Logger and suppresses or demotes specific non-critical errors.
type FilteredLogger struct {
	waLog.Logger
	module string
}

func NewFilteredLogger(logger waLog.Logger, module string) *FilteredLogger {
	return &FilteredLogger{
		Logger: logger,
		module: module,
	}
}

func (f *FilteredLogger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Demote common websocket EOF errors which are automatically handled by reconnections.
	if strings.Contains(msg, "Error reading from websocket") && (strings.Contains(msg, "EOF") || strings.Contains(msg, "connection reset by peer")) {
		f.Logger.Infof("Websocket connection lost (%s), whatsmeow will reconnect automatically.", msg)
		return
	}
	f.Logger.Errorf(format, args...)
}

func (f *FilteredLogger) Sub(module string) waLog.Logger {
	newModule := module
	if f.module != "" {
		newModule = f.module + "/" + module
	}
	return NewFilteredLogger(f.Logger.Sub(module), newModule)
}
