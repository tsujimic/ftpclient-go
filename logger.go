package ftpclient

import (
	"log"
	"os"
)

// Logger ...
type Logger interface {
	Log(v ...interface{})
	Logf(format string, v ...interface{})
}

// NewDefaultLogger ...
func NewDefaultLogger() Logger {
	return &defaultLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

type defaultLogger struct {
	logger *log.Logger
}

func (l defaultLogger) Log(args ...interface{}) {
	l.logger.Println(args...)
}

func (l defaultLogger) Logf(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}
