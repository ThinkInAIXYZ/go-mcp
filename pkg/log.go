package pkg

import (
	"fmt"
	"log"
	"os"
)

type Logger interface {
	Debugf(format string, a ...any)
	Infof(format string, a ...any)
	Warnf(format string, a ...any)
	Errorf(format string, a ...any)
}
type LogLevel uint32

const (
	LogLevelDebug = LogLevel(0)
	LogLevelInfo  = LogLevel(1)
	LogLevelWarn  = LogLevel(2)
	LogLevelError = LogLevel(3)
)

var DefaultLogger Logger = &defaultLogger{
	logLevel: LogLevelInfo,
	infoLog:  log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile), // Stdio transport send log information to Stdout, there may be problems.
	errLog:   log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile),
}

var DebugLogger Logger = &defaultLogger{
	logLevel: LogLevelDebug,
	infoLog:  log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
	errLog:   log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile),
}

type defaultLogger struct {
	logLevel LogLevel
	infoLog  *log.Logger
	errLog   *log.Logger
}

func (l *defaultLogger) Debugf(format string, a ...any) {
	if l.logLevel > LogLevelDebug {
		return
	}
	_ = l.infoLog.Output(2, fmt.Sprintf("[Debug] "+format, a...))
}

func (l *defaultLogger) Infof(format string, a ...any) {
	if l.logLevel > LogLevelInfo {
		return
	}
	_ = l.infoLog.Output(2, fmt.Sprintf("[Info] "+format, a...))
}

func (l *defaultLogger) Warnf(format string, a ...any) {
	if l.logLevel > LogLevelWarn {
		return
	}
	_ = l.errLog.Output(2, fmt.Sprintf("[Warn] "+format, a...))
}

func (l *defaultLogger) Errorf(format string, a ...any) {
	if l.logLevel > LogLevelError {
		return
	}
	_ = l.errLog.Output(2, fmt.Sprintf("[Error] "+format, a...))
}
