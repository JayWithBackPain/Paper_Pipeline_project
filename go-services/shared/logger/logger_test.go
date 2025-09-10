package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// captureOutput captures stdout for testing log output
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	f()
	
	w.Close()
	os.Stdout = old
	
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestNew(t *testing.T) {
	logger := New("test-service")
	
	if logger.serviceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", logger.serviceName)
	}
	
	if logger.requestID != "" {
		t.Errorf("Expected empty request ID, got '%s'", logger.requestID)
	}
	
	if logger.traceID != "" {
		t.Errorf("Expected empty trace ID, got '%s'", logger.traceID)
	}
}

func TestWithContext(t *testing.T) {
	logger := New("test-service")
	ctx := context.Background()
	
	contextLogger := logger.WithContext(ctx)
	
	if contextLogger.serviceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", contextLogger.serviceName)
	}
	
	// Should be a new instance
	if logger == contextLogger {
		t.Error("Expected new logger instance, got same instance")
	}
}

func TestWithTraceID(t *testing.T) {
	logger := New("test-service")
	traceID := "trace-123"
	
	tracedLogger := logger.WithTraceID(traceID)
	
	if tracedLogger.traceID != traceID {
		t.Errorf("Expected trace ID '%s', got '%s'", traceID, tracedLogger.traceID)
	}
	
	if tracedLogger.serviceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", tracedLogger.serviceName)
	}
}

func TestInfo(t *testing.T) {
	logger := New("test-service")
	message := "Test info message"
	
	output := captureOutput(func() {
		logger.Info(message)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.Level != LevelInfo {
		t.Errorf("Expected level INFO, got %s", logEntry.Level)
	}
	
	if logEntry.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, logEntry.Message)
	}
	
	if logEntry.Service != "test-service" {
		t.Errorf("Expected service 'test-service', got '%s'", logEntry.Service)
	}
	
	if logEntry.Timestamp == "" {
		t.Error("Expected timestamp to be set")
	}
}

func TestInfoWithCount(t *testing.T) {
	logger := New("test-service")
	message := "Processed data"
	count := 42
	
	output := captureOutput(func() {
		logger.InfoWithCount(message, count)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.DataCount == nil {
		t.Error("Expected data count to be set")
	} else if *logEntry.DataCount != count {
		t.Errorf("Expected data count %d, got %d", count, *logEntry.DataCount)
	}
}

func TestInfoWithDuration(t *testing.T) {
	logger := New("test-service")
	message := "Operation completed"
	duration := 1500 * time.Millisecond
	
	output := captureOutput(func() {
		logger.InfoWithDuration(message, duration)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.Duration == nil {
		t.Error("Expected duration to be set")
	} else if *logEntry.Duration != 1500 {
		t.Errorf("Expected duration 1500ms, got %dms", *logEntry.Duration)
	}
}

func TestWarn(t *testing.T) {
	logger := New("test-service")
	message := "Test warning message"
	
	output := captureOutput(func() {
		logger.Warn(message)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.Level != LevelWarn {
		t.Errorf("Expected level WARN, got %s", logEntry.Level)
	}
	
	if logEntry.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, logEntry.Message)
	}
}

func TestError(t *testing.T) {
	logger := New("test-service")
	message := "Test error message"
	testErr := errors.New("test error")
	
	output := captureOutput(func() {
		logger.Error(message, testErr)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.Level != LevelError {
		t.Errorf("Expected level ERROR, got %s", logEntry.Level)
	}
	
	if logEntry.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, logEntry.Message)
	}
	
	if logEntry.Error == nil {
		t.Error("Expected error details to be set")
	} else {
		if logEntry.Error.Message != "test error" {
			t.Errorf("Expected error message 'test error', got '%s'", logEntry.Error.Message)
		}
		if logEntry.Error.Type != "*errors.errorString" {
			t.Errorf("Expected error type '*errors.errorString', got '%s'", logEntry.Error.Type)
		}
	}
}

func TestErrorWithNilError(t *testing.T) {
	logger := New("test-service")
	message := "Test error message without error"
	
	output := captureOutput(func() {
		logger.Error(message, nil)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.Error != nil {
		t.Error("Expected error details to be nil when no error provided")
	}
}

func TestDebug(t *testing.T) {
	logger := New("test-service")
	message := "Test debug message"
	
	// Test without DEBUG log level
	output := captureOutput(func() {
		logger.Debug(message)
	})
	
	if strings.TrimSpace(output) != "" {
		t.Error("Expected no debug output when LOG_LEVEL is not DEBUG")
	}
	
	// Test with DEBUG log level
	os.Setenv("LOG_LEVEL", "DEBUG")
	defer os.Unsetenv("LOG_LEVEL")
	
	output = captureOutput(func() {
		logger.Debug(message)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.Level != LevelDebug {
		t.Errorf("Expected level DEBUG, got %s", logEntry.Level)
	}
}

func TestLogWithMetadata(t *testing.T) {
	logger := New("test-service")
	message := "Test message with metadata"
	metadata := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}
	
	output := captureOutput(func() {
		logger.Info(message, metadata)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.Metadata == nil {
		t.Error("Expected metadata to be set")
	} else {
		if logEntry.Metadata["key1"] != "value1" {
			t.Errorf("Expected metadata key1 'value1', got '%v'", logEntry.Metadata["key1"])
		}
		if logEntry.Metadata["key2"] != float64(42) { // JSON unmarshals numbers as float64
			t.Errorf("Expected metadata key2 42, got '%v'", logEntry.Metadata["key2"])
		}
		if logEntry.Metadata["key3"] != true {
			t.Errorf("Expected metadata key3 true, got '%v'", logEntry.Metadata["key3"])
		}
	}
}

func TestLogWithTraceID(t *testing.T) {
	logger := New("test-service")
	traceID := "trace-123"
	message := "Test message with trace ID"
	
	tracedLogger := logger.WithTraceID(traceID)
	
	output := captureOutput(func() {
		tracedLogger.Info(message)
	})
	
	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}
	
	if logEntry.TraceID != traceID {
		t.Errorf("Expected trace ID '%s', got '%s'", traceID, logEntry.TraceID)
	}
}

func TestGetRequestIDFromContext(t *testing.T) {
	// Test with nil context
	requestID := getRequestIDFromContext(nil)
	if requestID != "" {
		t.Errorf("Expected empty request ID for nil context, got '%s'", requestID)
	}
	
	// Test with background context
	ctx := context.Background()
	requestID = getRequestIDFromContext(ctx)
	if requestID != "" {
		t.Errorf("Expected empty request ID for background context, got '%s'", requestID)
	}
}

// Benchmark tests
func BenchmarkLogInfo(b *testing.B) {
	logger := New("test-service")
	
	// Redirect stdout to discard output during benchmark
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark test message")
	}
}

func BenchmarkLogError(b *testing.B) {
	logger := New("test-service")
	testErr := errors.New("benchmark error")
	
	// Redirect stdout to discard output during benchmark
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Error("Benchmark error message", testErr)
	}
}