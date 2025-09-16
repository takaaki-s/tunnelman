// Package core provides core logging functionality for SSH tunnel management.
package core

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	// LogLevelDebug is for verbose debugging information
	LogLevelDebug LogLevel = iota
	// LogLevelInfo is for informational messages
	LogLevelInfo
	// LogLevelWarn is for warning messages
	LogLevelWarn
	// LogLevelError is for error messages
	LogLevelError
)

// Logger provides structured logging with multiple levels
type Logger struct {
	mu       sync.RWMutex
	level    LogLevel
	output   io.Writer
	debugOut io.Writer
	prefix   string
	debug    bool
}

var (
	// DefaultLogger is the global logger instance
	DefaultLogger *Logger
	once          sync.Once
)

// InitLogger initializes the global logger
func InitLogger(debug bool) {
	once.Do(func() {
		DefaultLogger = NewLogger(debug)
	})
}

// NewLogger creates a new logger instance
func NewLogger(debug bool) *Logger {
	level := LogLevelInfo
	if debug {
		level = LogLevelDebug
	}

	return &Logger{
		level:    level,
		output:   os.Stderr,
		debugOut: os.Stderr,
		debug:    debug,
		prefix:   "",
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput sets the output writer for standard logs
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// SetDebugOutput sets the output writer for debug logs
func (l *Logger) SetDebugOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugOut = w
}

// SetPrefix sets a prefix for all log messages
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// formatMessage formats a log message with level and timestamp
func (l *Logger) formatMessage(level LogLevel, format string, args ...interface{}) string {
	levelStr := l.levelString(level)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	if l.prefix != "" {
		return fmt.Sprintf("[%s] %s [%s] %s", timestamp, levelStr, l.prefix, message)
	}
	return fmt.Sprintf("[%s] %s %s", timestamp, levelStr, message)
}

// levelString returns the string representation of a log level
func (l *Logger) levelString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// shouldLog checks if a message should be logged based on the current level
func (l *Logger) shouldLog(level LogLevel) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.level
}

// log writes a log message if the level is enabled
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	l.mu.RLock()
	output := l.output
	if level == LogLevelDebug && l.debugOut != nil {
		output = l.debugOut
	}
	l.mu.RUnlock()

	message := l.formatMessage(level, format, args...)
	fmt.Fprintln(output, message)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LogLevelDebug, format, args...)
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LogLevelInfo, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LogLevelWarn, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LogLevelError, format, args...)
}

// SSHCommand logs an SSH command in debug mode
func (l *Logger) SSHCommand(tunnelName string, cmd []string) {
	if !l.shouldLog(LogLevelDebug) {
		return
	}
	l.Debug("SSH command for tunnel '%s': %v", tunnelName, cmd)
}

// SSHOutput logs SSH command output in debug mode
func (l *Logger) SSHOutput(tunnelName string, stdout, stderr string) {
	if !l.shouldLog(LogLevelDebug) {
		return
	}
	if stdout != "" {
		l.Debug("SSH stdout for tunnel '%s': %s", tunnelName, stdout)
	}
	if stderr != "" {
		l.Debug("SSH stderr for tunnel '%s': %s", tunnelName, stderr)
	}
}

// Package-level convenience functions

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Debug(format, args...)
	} else {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs an informational message using the default logger
func Info(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Info(format, args...)
	} else {
		log.Printf("[INFO] "+format, args...)
	}
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Warn(format, args...)
	} else {
		log.Printf("[WARN] "+format, args...)
	}
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Error(format, args...)
	} else {
		log.Printf("[ERROR] "+format, args...)
	}
}

// LogSSHCommand logs an SSH command using the default logger
func LogSSHCommand(tunnelName string, cmd []string) {
	if DefaultLogger != nil {
		DefaultLogger.SSHCommand(tunnelName, cmd)
	}
}

// LogSSHOutput logs SSH command output using the default logger
func LogSSHOutput(tunnelName string, stdout, stderr string) {
	if DefaultLogger != nil {
		DefaultLogger.SSHOutput(tunnelName, stdout, stderr)
	}
}