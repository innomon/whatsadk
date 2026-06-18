package whatsapp

import (
	"github.com/innomon/whatsadk/internal/logger"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func NewFilteredLogger(module string) waLog.Logger {
	return logger.NewWhatsMeowLogger(nil, module)
}
