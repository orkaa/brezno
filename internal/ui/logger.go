package ui

import (
	"fmt"
	"os"
)

// Logger provides color-coded logging similar to bash scripts
type Logger struct {
	Verbose bool
	Quiet   bool
	NoColor bool
}

// NewLogger creates a new logger
func NewLogger(verbose, quiet, noColor bool) *Logger {
	return &Logger{
		Verbose: verbose,
		Quiet:   quiet,
		NoColor: noColor,
	}
}

// Color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

func (l *Logger) colorize(color, text string) string {
	if l.NoColor {
		return text
	}
	return color + text + colorReset
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	if l.Quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s\n", l.colorize(colorBlue, "[INFO] "+msg))
}

// Success logs a success message
func (l *Logger) Success(format string, args ...interface{}) {
	if l.Quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s\n", l.colorize(colorGreen, "[SUCCESS] "+msg))
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s\n", l.colorize(colorYellow, "[WARNING] "+msg))
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s\n", l.colorize(colorRed, "[ERROR] "+msg))
}

// Debug logs a debug message (only if verbose is enabled)
func (l *Logger) Debug(format string, args ...interface{}) {
	if !l.Verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s\n", l.colorize(colorCyan, "[DEBUG] "+msg))
}
