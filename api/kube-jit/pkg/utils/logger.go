package utils

import (
	"go.uber.org/zap"
)

var (
	logger *zap.Logger
)

// InitLogger sets the zap logger for this package
func InitLogger(l *zap.Logger) {
	logger = l
}
