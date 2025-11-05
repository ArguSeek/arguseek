package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id,omitempty"`
	Service   string                 `json:"service"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type Logger struct {
	serviceName string
	minLevel    Level
}

func New(serviceName string, level Level) *Logger {
	return &Logger{
		serviceName: serviceName,
		minLevel:    level,
	}
}

func (l *Logger) log(ctx context.Context, level Level, msg string, fields map[string]interface{}) {
	if level < l.minLevel {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   msg,
		Service:   l.serviceName,
		Fields:    fields,
	}

	// Extract request ID from context if available
	if requestID, ok := ctx.Value("requestID").(string); ok {
		entry.RequestID = requestID
	}

	jsonEntry, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}

	fmt.Println(string(jsonEntry))

	// For fatal logs, exit the program
	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, DEBUG, msg, f)
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, INFO, msg, f)
}

func (l *Logger) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, WARN, msg, f)
}

func (l *Logger) Error(ctx context.Context, msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, ERROR, msg, f)
}

func (l *Logger) Fatal(ctx context.Context, msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, FATAL, msg, f)
}

// Default logger instance
var defaultLogger = New("arguseek", INFO)

// Package level functions for convenience
func Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	defaultLogger.Debug(ctx, msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	defaultLogger.Info(ctx, msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
	defaultLogger.Warn(ctx, msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...map[string]interface{}) {
	defaultLogger.Error(ctx, msg, fields...)
}

func Fatal(ctx context.Context, msg string, fields ...map[string]interface{}) {
	defaultLogger.Fatal(ctx, msg, fields...)
}

// SetLevel sets the minimum log level for the default logger
func SetLevel(level Level) {
	defaultLogger.minLevel = level
}
