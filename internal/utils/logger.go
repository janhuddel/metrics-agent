// Package utils provides logging utilities with configurable levels.
package utils

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
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
	mu       sync.RWMutex
	level    LogLevel
	debugLog *log.Logger
	infoLog  *log.Logger
	warnLog  *log.Logger
	errorLog *log.Logger
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
	flags := log.LstdFlags | log.Lmicroseconds

	return &Logger{
		level:    level,
		debugLog: log.New(output, "[DEBUG] ", flags),
		infoLog:  log.New(output, "[INFO] ", flags),
		warnLog:  log.New(output, "[WARN] ", flags),
		errorLog: log.New(output, "[ERROR] ", flags),
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
	l.debugLog.SetOutput(output)
	l.infoLog.SetOutput(output)
	l.warnLog.SetOutput(output)
	l.errorLog.SetOutput(output)
}

// Debug logs a debug message
func (l *Logger) Debug(v ...interface{}) {
	if l.shouldLog(DEBUG) {
		l.debugLog.Print(v...)
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.shouldLog(DEBUG) {
		l.debugLog.Printf(format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(v ...interface{}) {
	if l.shouldLog(INFO) {
		l.infoLog.Print(v...)
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, v ...interface{}) {
	if l.shouldLog(INFO) {
		l.infoLog.Printf(format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(v ...interface{}) {
	if l.shouldLog(WARN) {
		l.warnLog.Print(v...)
	}
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, v ...interface{}) {
	if l.shouldLog(WARN) {
		l.warnLog.Printf(format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(v ...interface{}) {
	if l.shouldLog(ERROR) {
		l.errorLog.Print(v...)
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.shouldLog(ERROR) {
		l.errorLog.Printf(format, v...)
	}
}

// Fatal logs a fatal error message and exits
func (l *Logger) Fatal(v ...interface{}) {
	l.errorLog.Print(v...)
	os.Exit(1)
}

// Fatalf logs a formatted fatal error message and exits
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.errorLog.Printf(format, v...)
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
	GetLogger().Debug(v...)
}

// Debugf logs a formatted debug message using the global logger
func Debugf(format string, v ...interface{}) {
	GetLogger().Debugf(format, v...)
}

// Info logs an info message using the global logger
func Info(v ...interface{}) {
	GetLogger().Info(v...)
}

// Infof logs a formatted info message using the global logger
func Infof(format string, v ...interface{}) {
	GetLogger().Infof(format, v...)
}

// Warn logs a warning message using the global logger
func Warn(v ...interface{}) {
	GetLogger().Warn(v...)
}

// Warnf logs a formatted warning message using the global logger
func Warnf(format string, v ...interface{}) {
	GetLogger().Warnf(format, v...)
}

// Error logs an error message using the global logger
func Error(v ...interface{}) {
	GetLogger().Error(v...)
}

// Errorf logs a formatted error message using the global logger
func Errorf(format string, v ...interface{}) {
	GetLogger().Errorf(format, v...)
}

// Fatal logs a fatal error message and exits using the global logger
func Fatal(v ...interface{}) {
	GetLogger().Fatal(v...)
}

// Fatalf logs a formatted fatal error message and exits using the global logger
func Fatalf(format string, v ...interface{}) {
	GetLogger().Fatalf(format, v...)
}
