package logger

import (
	"fmt"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeAPI       ErrorType = "API_ERROR"
	ErrorTypeS3        ErrorType = "S3_ERROR"
	ErrorTypeConfig    ErrorType = "CONFIG_ERROR"
	ErrorTypeData      ErrorType = "DATA_ERROR"
	ErrorTypeInternal  ErrorType = "INTERNAL_ERROR"
)

// AppError represents an application-specific error with context
type AppError struct {
	Type     ErrorType
	Message  string
	Code     string
	Cause    error
	Metadata map[string]interface{}
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError creates a new application error
func NewAppError(errorType ErrorType, message string, cause error) *AppError {
	return &AppError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}

// NewAppErrorWithCode creates a new application error with an error code
func NewAppErrorWithCode(errorType ErrorType, message, code string, cause error) *AppError {
	return &AppError{
		Type:    errorType,
		Message: message,
		Code:    code,
		Cause:   cause,
	}
}

// NewAppErrorWithMetadata creates a new application error with metadata
func NewAppErrorWithMetadata(errorType ErrorType, message string, cause error, metadata map[string]interface{}) *AppError {
	return &AppError{
		Type:     errorType,
		Message:  message,
		Cause:    cause,
		Metadata: metadata,
	}
}

// ErrorHandler provides centralized error handling and logging
type ErrorHandler struct {
	logger *Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// Handle processes an error and logs it appropriately
func (eh *ErrorHandler) Handle(err error, context string) error {
	if err == nil {
		return nil
	}
	
	// Check if it's an AppError
	if appErr, ok := err.(*AppError); ok {
		eh.logger.Error(
			fmt.Sprintf("%s: %s", context, appErr.Message),
			err,
			appErr.Metadata,
		)
		return appErr
	}
	
	// Handle generic errors
	eh.logger.Error(fmt.Sprintf("%s: unexpected error", context), err)
	return NewAppError(ErrorTypeInternal, context, err)
}

// HandleWithRecovery handles panics and converts them to errors
func (eh *ErrorHandler) HandleWithRecovery(context string) error {
	if r := recover(); r != nil {
		err := fmt.Errorf("panic recovered: %v", r)
		eh.logger.Error(fmt.Sprintf("%s: panic occurred", context), err)
		return NewAppError(ErrorTypeInternal, "panic recovered", err)
	}
	return nil
}

// WrapError wraps an existing error with additional context
func WrapError(err error, errorType ErrorType, message string) error {
	if err == nil {
		return nil
	}
	return NewAppError(errorType, message, err)
}

// IsErrorType checks if an error is of a specific type
func IsErrorType(err error, errorType ErrorType) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == errorType
	}
	return false
}