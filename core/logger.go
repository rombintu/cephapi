package core

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type CustomFormat struct {
	customField string
	logrus.TextFormatter
}

func (f *CustomFormat) Format(entry *logrus.Entry) ([]byte, error) {
	l, err := f.TextFormatter.Format(entry)
	return append([]byte(fmt.Sprintf("[%s] ", f.customField)), l...), err
}

func NewLogger(cf string) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&CustomFormat{
		cf,
		logrus.TextFormatter{
			DisableColors:   false,
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		},
	})
	return logger
}
