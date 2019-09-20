package logger

import (
	"log"

	"go.uber.org/zap"
)

// Log is a global pointer to zap logger
var zapLogger *zap.Logger

// Debug ...
func Debug(format string, fields ...zap.Field) {
	zapLogger.Debug(format, fields...)
}

// Info ...
func Info(format string, fields ...zap.Field) {
	zapLogger.Info(format, fields...)
}

// Warn ...
func Warn(format string, fields ...zap.Field) {
	zapLogger.Warn(format, fields...)
}

// Error ..
func Error(format string, fields ...zap.Field) {
	zapLogger.Error(format, fields...)
}

// Fatal ...
func Fatal(format string, fields ...zap.Field) {
	zapLogger.Fatal(format, fields...)
}

// Initializa the logger
func init() {
	l, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize the logger: %v", err)
	}

	zapLogger = l
}
