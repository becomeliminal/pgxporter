package logging

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// Default log format - rich.
	richLogFormat = "%time%:%file%:%line%# [%level%] - %message%\n"

	// Default timestamp format
	defaultTimestampFormat = time.StampNano
)

// Formatter implements the logrus.Formatter interface.
// It has a very nice
type Formatter struct {
	// Timestamp format
	TimestampFormat string
	// Available standard keys: time, msg, lvl
	// Also can include custom fields but limited to strings.
	// All of fields need to be wrapped inside %% i.e %time% %msg%
	LogFormat string
}

// Format implements a logrus formatter's Format method.
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	output := f.LogFormat
	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}

	output = strings.Replace(output, "%time%", entry.Time.Format(timestampFormat), 1)
	output = strings.Replace(output, "%file%", filepath.Base(entry.Caller.File), 1)
	output = strings.Replace(output, "%line%", strconv.Itoa(entry.Caller.Line), 1)
	output = strings.Replace(output, "%message%", entry.Message, 1)
	output = strings.Replace(output, "%level%", strings.ToUpper(entry.Level.String()), 1)

	for k, v := range entry.Data {
		if s, ok := v.(string); ok {
			output = strings.Replace(output, "%"+k+"%", s, 1)
		}
	}

	return []byte(output), nil
}
