package handlers

import (
	"go.uber.org/zap"
)

var (
	logger *zap.Logger
)

func InitLogger(l *zap.Logger) {
	logger = l
}
