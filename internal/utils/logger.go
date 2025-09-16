// Package utils provides logging utilities with configurable levels.
package utils

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the different logging levels
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides a structured logger with configurable levels
type Logger struct {
	mu     sync.RWMutex
	level  LogLevel
	output io.Writer
}

var (
	// Global logger instance
	globalLogger *Logger
	once         sync.Once
)

// GetGlobalLogger returns the global logger instance (for testing purposes)
func GetGlobalLogger() *Logger {
	once.Do(func() {
		globalLogger = NewLogger(INFO, os.Stderr)
	})
	return globalLogger
}

// SetGlobalLogger sets the global logger instance (for testing purposes)
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	once.Do(func() {
		globalLogger = NewLogger(INFO, os.Stderr)
	})
	return globalLogger
}

// NewLogger creates a new logger with the specified level and output writer
func NewLogger(level LogLevel, output io.Writer) *Logger {
	return &Logger{
		level:  level,
		output: output,
	}
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current logging level
func (l *Logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// SetOutput sets the output writer for all log levels
func (l *Logger) SetOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = output
}

// getCallerInfo gets the caller information for logging
func getCallerInfo() (string, int) {
	// Try different call depths to find the actual caller
	for depth := 2; depth <= 5; depth++ {
		_, file, line, ok := runtime.Caller(depth)
		if !ok {
			continue
		}
		// Skip internal Go runtime files and our logger files
		if !strings.Contains(file, "runtime/") &&
			!strings.Contains(file, "proc.go") &&
			!strings.Contains(file, "asm_") &&
			!strings.Contains(file, "logger.go") {
			// Extract just the filename from the full path
			parts := strings.Split(file, "/")
			filename := parts[len(parts)-1]
			return filename, line
		}
	}
	return "unknown", 0
}

// formatLogMessage formats a log message with the custom format:
// timestamp [loglevel] [filename:line_no] message
func (l *Logger) formatLogMessage(level LogLevel, message string) string {
	filename, line := getCallerInfo()

	// Format timestamp
	timestamp := time.Now().Format("2006/01/02 15:04:05.000000")

	// Format the log message with fixed-width log level (5 characters)
	return fmt.Sprintf("%s [%-5s] [%s:%d] %s\n", timestamp, level.String(), filename, line, message)
}

// Debug logs a debug message
func (l *Logger) Debug(v ...interface{}) {
	if l.shouldLog(DEBUG) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(DEBUG, fmt.Sprint(v...)))
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.shouldLog(DEBUG) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(DEBUG, fmt.Sprintf(format, v...)))
	}
}

// Info logs an info message
func (l *Logger) Info(v ...interface{}) {
	if l.shouldLog(INFO) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(INFO, fmt.Sprint(v...)))
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, v ...interface{}) {
	if l.shouldLog(INFO) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(INFO, fmt.Sprintf(format, v...)))
	}
}

// Warn logs a warning message
func (l *Logger) Warn(v ...interface{}) {
	if l.shouldLog(WARN) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(WARN, fmt.Sprint(v...)))
	}
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, v ...interface{}) {
	if l.shouldLog(WARN) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(WARN, fmt.Sprintf(format, v...)))
	}
}

// Error logs an error message
func (l *Logger) Error(v ...interface{}) {
	if l.shouldLog(ERROR) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(ERROR, fmt.Sprint(v...)))
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.shouldLog(ERROR) {
		l.mu.RLock()
		output := l.output
		l.mu.RUnlock()
		fmt.Fprint(output, l.formatLogMessage(ERROR, fmt.Sprintf(format, v...)))
	}
}

// Fatal logs a fatal error message and exits
func (l *Logger) Fatal(v ...interface{}) {
	l.mu.RLock()
	output := l.output
	l.mu.RUnlock()
	fmt.Fprint(output, l.formatLogMessage(ERROR, fmt.Sprint(v...)))
	os.Exit(1)
}

// Fatalf logs a formatted fatal error message and exits
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.mu.RLock()
	output := l.output
	l.mu.RUnlock()
	fmt.Fprint(output, l.formatLogMessage(ERROR, fmt.Sprintf(format, v...)))
	os.Exit(1)
}

// shouldLog checks if the given level should be logged
func (l *Logger) shouldLog(level LogLevel) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.level
}

// ParseLogLevel parses a string log level into a LogLevel constant
func ParseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO // Default to INFO level
	}
}

// Global convenience functions that use the global logger

// SetGlobalLogLevel sets the global logger level
func SetGlobalLogLevel(level LogLevel) {
	GetLogger().SetLevel(level)
}

// SetGlobalLogLevelFromString sets the global logger level from a string
func SetGlobalLogLevelFromString(level string) {
	SetGlobalLogLevel(ParseLogLevel(level))
}

// Debug logs a debug message using the global logger
func Debug(v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(DEBUG) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(DEBUG, fmt.Sprint(v...)))
	}
}

// Debugf logs a formatted debug message using the global logger
func Debugf(format string, v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(DEBUG) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(DEBUG, fmt.Sprintf(format, v...)))
	}
}

// Info logs an info message using the global logger
func Info(v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(INFO) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(INFO, fmt.Sprint(v...)))
	}
}

// Infof logs a formatted info message using the global logger
func Infof(format string, v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(INFO) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(INFO, fmt.Sprintf(format, v...)))
	}
}

// Warn logs a warning message using the global logger
func Warn(v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(WARN) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(WARN, fmt.Sprint(v...)))
	}
}

// Warnf logs a formatted warning message using the global logger
func Warnf(format string, v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(WARN) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(WARN, fmt.Sprintf(format, v...)))
	}
}

// Error logs an error message using the global logger
func Error(v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(ERROR) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(ERROR, fmt.Sprint(v...)))
	}
}

// Errorf logs a formatted error message using the global logger
func Errorf(format string, v ...interface{}) {
	logger := GetLogger()
	if logger.shouldLog(ERROR) {
		logger.mu.RLock()
		output := logger.output
		logger.mu.RUnlock()
		fmt.Fprint(output, logger.formatLogMessage(ERROR, fmt.Sprintf(format, v...)))
	}
}

// Fatal logs a fatal error message and exits using the global logger
func Fatal(v ...interface{}) {
	logger := GetLogger()
	logger.mu.RLock()
	output := logger.output
	logger.mu.RUnlock()
	fmt.Fprint(output, logger.formatLogMessage(ERROR, fmt.Sprint(v...)))
	os.Exit(1)
}

// Fatalf logs a formatted fatal error message and exits using the global logger
func Fatalf(format string, v ...interface{}) {
	logger := GetLogger()
	logger.mu.RLock()
	output := logger.output
	logger.mu.RUnlock()
	fmt.Fprint(output, logger.formatLogMessage(ERROR, fmt.Sprintf(format, v...)))
	os.Exit(1)
}
