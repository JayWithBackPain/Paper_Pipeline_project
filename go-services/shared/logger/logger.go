package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
	LevelDebug LogLevel = "DEBUG"
)

// LogEntry represents a structured log entry for CloudWatch
type LogEntry struct {
	Timestamp   string                 `json:"timestamp"`
	Level       LogLevel               `json:"level"`
	Message     string                 `json:"message"`
	Service     string                 `json:"service"`
	TraceID     string                 `json:"trace_id,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	Duration    *int64                 `json:"duration_ms,omitempty"`
	DataCount   *int                   `json:"data_count,omitempty"`
	Error       *ErrorDetails          `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorDetails provides structured error information
type ErrorDetails struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Logger provides structured logging functionality
type Logger struct {
	serviceName string
	requestID   string
	traceID     string
}

// New creates a new structured logger instance
func New(serviceName string) *Logger {
	return &Logger{
		serviceName: serviceName,
	}
}

// WithContext adds context information to the logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	newLogger := &Logger{
		serviceName: l.serviceName,
		requestID:   l.requestID,
		traceID:     l.traceID,
	}
	
	// Extract AWS Lambda request ID from context if available
	if requestID := getRequestIDFromContext(ctx); requestID != "" {
		newLogger.requestID = requestID
	}
	
	return newLogger
}

// WithTraceID adds a trace ID to the logger
func (l *Logger) WithTraceID(traceID string) *Logger {
	newLogger := *l
	newLogger.traceID = traceID
	return &newLogger
}

// Info logs an informational message
func (l *Logger) Info(message string, metadata ...map[string]interface{}) {
	l.log(LevelInfo, message, nil, nil, nil, metadata...)
}

// InfoWithCount logs an informational message with data count
func (l *Logger) InfoWithCount(message string, count int, metadata ...map[string]interface{}) {
	l.log(LevelInfo, message, nil, &count, nil, metadata...)
}

// InfoWithDuration logs an informational message with duration
func (l *Logger) InfoWithDuration(message string, duration time.Duration, metadata ...map[string]interface{}) {
	durationMs := duration.Milliseconds()
	l.log(LevelInfo, message, &durationMs, nil, nil, metadata...)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, metadata ...map[string]interface{}) {
	l.log(LevelWarn, message, nil, nil, nil, metadata...)
}

// Error logs an error message
func (l *Logger) Error(message string, err error, metadata ...map[string]interface{}) {
	var errorDetails *ErrorDetails
	if err != nil {
		errorDetails = &ErrorDetails{
			Type:    fmt.Sprintf("%T", err),
			Message: err.Error(),
		}
	}
	l.log(LevelError, message, nil, nil, errorDetails, metadata...)
}

// Debug logs a debug message (only in development)
func (l *Logger) Debug(message string, metadata ...map[string]interface{}) {
	if os.Getenv("LOG_LEVEL") == "DEBUG" {
		l.log(LevelDebug, message, nil, nil, nil, metadata...)
	}
}

// log is the internal logging method that outputs structured JSON
func (l *Logger) log(level LogLevel, message string, duration *int64, dataCount *int, errorDetails *ErrorDetails, metadata ...map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Service:   l.serviceName,
		TraceID:   l.traceID,
		RequestID: l.requestID,
		Duration:  duration,
		DataCount: dataCount,
		Error:     errorDetails,
	}
	
	// Merge metadata if provided
	if len(metadata) > 0 && metadata[0] != nil {
		entry.Metadata = metadata[0]
	}
	
	// Marshal to JSON and output
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		// Fallback to standard logging if JSON marshaling fails
		log.Printf("[%s] %s: %s (JSON marshal error: %v)", level, l.serviceName, message, err)
		return
	}
	
	// Output to stdout for CloudWatch Logs
	fmt.Println(string(jsonBytes))
}

// getRequestIDFromContext extracts AWS Lambda request ID from context
func getRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	
	// For now, return empty string as we don't have AWS Lambda context in tests
	return ""
}