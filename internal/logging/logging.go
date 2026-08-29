// Package logging provides a tiny structured file logger for The Purge. Every
// entry is a single line: "<RFC3339 timestamp> <LEVEL> <message>". The logger
// is best-effort: callers should never fail a command because logging failed.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger appends timestamped lines to a file, creating parent directories on
// open. It is safe for concurrent use.
type Logger struct {
	mu sync.Mutex
	f  *os.File
}

// New opens (creating if needed) the log file at path.
func New(path string) (*Logger, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f}, nil
}

// Info logs an informational event.
func (l *Logger) Info(format string, args ...any) { l.log("INFO", format, args...) }

// Warn logs a recoverable problem (skipped immutable file, unreadable item).
func (l *Logger) Warn(format string, args ...any) { l.log("WARN", format, args...) }

// Error logs a failure that prevented the intended action.
func (l *Logger) Error(format string, args ...any) { l.log("ERROR", format, args...) }

func (l *Logger) log(level, format string, args ...any) {
	line := fmt.Sprintf("%s %s %s\n", time.Now().Format(time.RFC3339), level, fmt.Sprintf(format, args...))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		l.f.WriteString(line)
	}
}

// Close flushes and closes the underlying file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}
