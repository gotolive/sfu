package logger

import (
	"fmt"
	"io"
	"log"
	"sync"
)

var (
	_ Logger = new(stubLogger)
	_ Logger = new(defaultLogger)
)

type Logger interface {
	Error(v ...interface{})
	Errorf(format string, v ...interface{})

	Warn(v ...interface{})
	Warnf(format string, v ...interface{})

	Info(v ...interface{})
	Infof(format string, v ...interface{})

	Debug(v ...interface{})
	Debugf(format string, v ...interface{})

	SetLevel(level int)
	SetOutput(w io.Writer)
}

const (
	LevelError = iota
	LevelWarn
	LevelInfo
	LevelDebug
	LevelAll
)

func NewLogger(level int, trace string) Logger {
	return &defaultLogger{
		level: level,
		trace: fmt.Sprintf("[%s]", trace),
	}
}

type stubLogger struct{}

func (l *stubLogger) SetLevel(_ int) {
}

func (l *stubLogger) SetOutput(_ io.Writer) {
}

func (l *stubLogger) Error(_ ...interface{}) {
}

func (l *stubLogger) Errorf(_ string, _ ...interface{}) {
}

func (l *stubLogger) Warn(_ ...interface{}) {
}

func (l *stubLogger) Warnf(_ string, _ ...interface{}) {
}

func (l *stubLogger) Info(_ ...interface{}) {
}

func (l *stubLogger) Infof(_ string, _ ...interface{}) {
}

func (l *stubLogger) Debug(_ ...interface{}) {
}

func (l *stubLogger) Debugf(_ string, _ ...interface{}) {
}

// Only for stub
type defaultLogger struct {
	// level is concurrent unsafe, read/write.
	level int
	trace string
	m     sync.Mutex
}

func (l *defaultLogger) SetLevel(level int) {
	l.m.Lock()
	l.level = level
	l.m.Unlock()
}

func (l *defaultLogger) SetOutput(w io.Writer) {
	log.SetOutput(w)
}

func (l *defaultLogger) Output(calldepth int, s string) {
	_ = log.Output(calldepth, s)
}

const defaultCallDepth = 3

func (l *defaultLogger) Error(v ...interface{}) {
	if l.level >= LevelError {
		v = append([]interface{}{l.trace, "[ERROR]"}, v...)
		s := fmt.Sprintln(v...)
		l.Output(defaultCallDepth, s)
	}
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	if l.level >= LevelWarn {
		l.Error(fmt.Sprintf(format, v...))
	}
}

func (l *defaultLogger) Warn(v ...interface{}) {
	if l.level >= LevelWarn {
		v = append([]interface{}{l.trace, "[WARN]"}, v...)
		s := fmt.Sprintln(v...)
		l.Output(defaultCallDepth, s)
	}
}

func (l *defaultLogger) Warnf(format string, v ...interface{}) {
	if l.level >= LevelWarn {
		l.Warn(fmt.Sprintf(format, v...))
	}
}

func (l *defaultLogger) Info(v ...interface{}) {
	if l.level >= LevelInfo {
		v = append([]interface{}{l.trace, "[INFO]"}, v...)
		s := fmt.Sprintln(v...)
		l.Output(defaultCallDepth, s)
	}
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	if l.level >= LevelInfo {
		l.Info(fmt.Sprintf(format, v...))
	}
}

func (l *defaultLogger) Debug(v ...interface{}) {
	if l.level >= LevelDebug {
		v = append([]interface{}{l.trace, "[DEBUG]"}, v...)
		s := fmt.Sprintln(v...)
		l.Output(defaultCallDepth, s)
	}
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	if l.level >= LevelDebug {
		l.Debug(fmt.Sprintf(format, v...))
	}
}

var std = &defaultLogger{
	level: LevelInfo,
	trace: fmt.Sprintf("[%s]", "STD"),
}

func Error(v ...interface{}) {
	if std.level >= LevelError {
		v = append([]interface{}{std.trace, "[ERROR]"}, v...)
		s := fmt.Sprintln(v...)
		std.Output(defaultCallDepth, s)
	}
}

func Errorf(format string, v ...interface{}) {
	if std.level >= LevelWarn {
		std.Errorf(fmt.Sprintf(format, v...))
	}
}

func Warn(v ...interface{}) {
	if std.level >= LevelWarn {
		v = append([]interface{}{std.trace, "[WARN]"}, v...)
		s := fmt.Sprintln(v...)
		std.Output(defaultCallDepth, s)
	}
}

func Warnf(format string, v ...interface{}) {
	if std.level >= LevelWarn {
		std.Warn(fmt.Sprintf(format, v...))
	}
}

func Info(v ...interface{}) {
	if std.level >= LevelInfo {
		v = append([]interface{}{std.trace, "[INFO]"}, v...)
		s := fmt.Sprintln(v...)
		std.Output(defaultCallDepth, s)
	}
}

func Infof(format string, v ...interface{}) {
	if std.level >= LevelInfo {
		v = append([]interface{}{std.trace, "[INFO]"}, fmt.Sprintf(format, v...))
		s := fmt.Sprintln(v...)
		std.Output(defaultCallDepth, s)
	}
}

func Debug(v ...interface{}) {
	if std.level >= LevelDebug {
		v = append([]interface{}{std.trace, "[DEBUG]"}, v...)
		s := fmt.Sprintln(v...)
		std.Output(defaultCallDepth, s)
	}
}

func Debugf(format string, v ...interface{}) {
	if std.level >= LevelDebug {
		std.Debug(fmt.Sprintf(format, v...))
	}
}

func SetLevel(level int) {
	std.SetLevel(level)
}

func SetOutput(w io.Writer) {
	std.SetOutput(w)
}
