package stderrlogger

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
)

// New returns a Logger which will write log messages to stderr.
func New() aws.Logger {
	return &stderrLogger{log.New(os.Stderr, "", log.LstdFlags)}
}

type stderrLogger struct {
	logger *log.Logger
}

func (l *stderrLogger) Log(args ...interface{}) {
	l.logger.Println(args...)
}
