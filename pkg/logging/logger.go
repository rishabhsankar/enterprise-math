// Package logging provides enterprise-grade logging infrastructure with
// multiple output formats, log levels, structured fields, and middleware support.
package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry in the enterprise logging hierarchy.
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// LogFormat specifies the serialization format for log entries.
type LogFormat int

const (
	TEXT LogFormat = iota
	JSON
	XML
)

// LogConfig holds the comprehensive configuration for the enterprise logger.
type LogConfig struct {
	Level        LogLevel
	Format       LogFormat
	OutputPath   string
	EnableCaller bool
	BufferSize   int
	FlushInterval time.Duration
}

// LogEntry represents a single structured log record with full context.
type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Caller     string                 `json:"caller,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

// Logger defines the interface for enterprise logging operations.
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
	Fatal(msg string, fields map[string]interface{})
	WithFields(fields map[string]interface{}) Logger
	WithPrefix(prefix string) Logger
}

// LogMiddleware defines a function that can intercept and transform log entries.
type LogMiddleware func(entry *LogEntry) *LogEntry

// EnterpriseLogger implements the Logger interface with full enterprise features.
type EnterpriseLogger struct {
	config      LogConfig
	fields      map[string]interface{}
	prefix      string
	middlewares []LogMiddleware
	mu          sync.RWMutex
	buffer      []*LogEntry
	writer      LogWriter
}

// LogWriter defines the interface for log output destinations.
type LogWriter interface {
	Write(entry *LogEntry) error
	Flush() error
	Close() error
}

// ConsoleLogWriter writes log entries to stdout or stderr.
type ConsoleLogWriter struct {
	format LogFormat
	output *os.File
}

// NewConsoleLogWriter creates a new console-based log writer.
func NewConsoleLogWriter(format LogFormat, outputPath string) *ConsoleLogWriter {
	output := os.Stdout
	if outputPath == "stderr" {
		output = os.Stderr
	}
	return &ConsoleLogWriter{format: format, output: output}
}

// Write outputs a log entry to the console in the configured format.
func (w *ConsoleLogWriter) Write(entry *LogEntry) error {
	switch w.format {
	case JSON:
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal log entry: %w", err)
		}
		_, err = fmt.Fprintln(w.output, string(data))
		return err
	case TEXT:
		_, err := fmt.Fprintf(w.output, "[%s] %s: %s %v\n",
			entry.Timestamp.Format(time.RFC3339),
			entry.Level,
			entry.Message,
			entry.Fields,
		)
		return err
	default:
		_, err := fmt.Fprintf(w.output, "<log level=\"%s\" time=\"%s\"><message>%s</message></log>\n",
			entry.Level,
			entry.Timestamp.Format(time.RFC3339),
			entry.Message,
		)
		return err
	}
}

// Flush ensures all buffered log entries are written.
func (w *ConsoleLogWriter) Flush() error { return nil }

// Close releases any resources held by the writer.
func (w *ConsoleLogWriter) Close() error { return nil }

// NewEnterpriseLogger creates a new enterprise logger with the given configuration.
func NewEnterpriseLogger(config LogConfig) *EnterpriseLogger {
	return &EnterpriseLogger{
		config:      config,
		fields:      make(map[string]interface{}),
		middlewares: make([]LogMiddleware, 0),
		buffer:      make([]*LogEntry, 0, config.BufferSize),
		writer:      NewConsoleLogWriter(config.Format, config.OutputPath),
	}
}

func levelToString(level LogLevel) string {
	switch level {
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

func (l *EnterpriseLogger) log(level LogLevel, msg string, fields map[string]interface{}) {
	if level < l.config.Level {
		return
	}

	entry := &LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     levelToString(level),
		Message:   msg,
		Fields:    l.mergeFields(fields),
	}

	if l.prefix != "" {
		entry.Message = fmt.Sprintf("[%s] %s", l.prefix, entry.Message)
	}

	if l.config.EnableCaller {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			entry.Caller = fmt.Sprintf("%s:%d", file, line)
		}
	}

	for _, mw := range l.middlewares {
		entry = mw(entry)
		if entry == nil {
			return
		}
	}

	_ = l.writer.Write(entry)
}

func (l *EnterpriseLogger) mergeFields(fields map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	l.mu.RLock()
	for k, v := range l.fields {
		merged[k] = v
	}
	l.mu.RUnlock()
	if fields != nil {
		for k, v := range fields {
			merged[k] = v
		}
	}
	return merged
}

// Debug logs a message at DEBUG level.
func (l *EnterpriseLogger) Debug(msg string, fields map[string]interface{}) { l.log(DEBUG, msg, fields) }

// Info logs a message at INFO level.
func (l *EnterpriseLogger) Info(msg string, fields map[string]interface{}) { l.log(INFO, msg, fields) }

// Warn logs a message at WARN level.
func (l *EnterpriseLogger) Warn(msg string, fields map[string]interface{}) { l.log(WARN, msg, fields) }

// Error logs a message at ERROR level.
func (l *EnterpriseLogger) Error(msg string, fields map[string]interface{}) { l.log(ERROR, msg, fields) }

// Fatal logs a message at FATAL level.
func (l *EnterpriseLogger) Fatal(msg string, fields map[string]interface{}) { l.log(FATAL, msg, fields) }

// WithFields returns a new logger with additional context fields.
func (l *EnterpriseLogger) WithFields(fields map[string]interface{}) Logger {
	newLogger := &EnterpriseLogger{
		config:      l.config,
		fields:      l.mergeFields(fields),
		prefix:      l.prefix,
		middlewares: l.middlewares,
		buffer:      l.buffer,
		writer:      l.writer,
	}
	return newLogger
}

// WithPrefix returns a new logger with a message prefix.
func (l *EnterpriseLogger) WithPrefix(prefix string) Logger {
	newLogger := &EnterpriseLogger{
		config:      l.config,
		fields:      l.mergeFields(nil),
		prefix:      prefix,
		middlewares: l.middlewares,
		buffer:      l.buffer,
		writer:      l.writer,
	}
	return newLogger
}

// AddMiddleware adds a log processing middleware to the chain.
func (l *EnterpriseLogger) AddMiddleware(mw LogMiddleware) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.middlewares = append(l.middlewares, mw)
}
