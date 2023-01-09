package core

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/fatih/color"
)

const (
	FATAL     = 5
	ERROR     = 4
	WARN      = 3
	IMPORTANT = 2
	INFO      = 1
	DEBUG     = 0
)

var LogColors = map[int]*color.Color{
	FATAL:     color.New(color.FgRed).Add(color.Bold),
	ERROR:     color.New(color.FgRed),
	WARN:      color.New(color.FgYellow),
	IMPORTANT: color.New(color.Bold),
	DEBUG:     color.New(color.FgCyan).Add(color.Faint),
}

var (
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintfFunc()
	red    = color.New(color.FgRed).SprintFunc()
)

type Logger struct {
	sync.Mutex

	debug   bool
	silent  bool
	noColor bool

	writer io.Writer
}

func NewLogger(writer io.Writer, debug, silent, noColor bool) *Logger {
	if writer == nil {
		writer = os.Stdout
	}
	return &Logger{
		debug:   debug,
		silent:  silent,
		noColor: noColor,
		writer:  writer,
	}
}

func (l *Logger) SetSilent(s bool) {
	l.silent = s
}

func (l *Logger) SetDebug(d bool) {
	l.debug = d
}

func (l *Logger) SetNoColor(nc bool) {
	l.noColor = nc
}

func (l *Logger) SetWriter(w io.Writer) {
	l.writer = w
}

func (l *Logger) Log(level int, format string, args ...interface{}) {
	l.Lock()
	defer l.Unlock()
	if level == DEBUG && !l.debug {
		return
	} else if level < ERROR && l.silent {
		return
	}

	if c, ok := LogColors[level]; ok && !l.noColor {
		_, err := c.Fprintf(l.writer, format, args...)
		if err != nil {
			_, _ = c.Printf(format, args...)
		}
	} else {
		_, err := fmt.Fprintf(l.writer, format, args...)
		if err != nil {
			fmt.Printf(format, args...)
		}
	}

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.Log(FATAL, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(ERROR, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.Log(WARN, format, args...)
}

func (l *Logger) Important(format string, args ...interface{}) {
	l.Log(IMPORTANT, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(INFO, format, args...)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.Log(DEBUG, format, args...)
}

func (l *Logger) Green(s string) string {
	if l.noColor {
		return s
	}
	return green(s)
}

func (l *Logger) Yellow(s string) string {
	if l.noColor {
		return s
	}
	return yellow(s)
}

func (l *Logger) Red(s string) string {
	if l.noColor {
		return s
	}
	return red(s)
}
