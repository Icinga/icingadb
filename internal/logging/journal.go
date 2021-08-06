package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-systemd/v22/journal"
	"go.etcd.io/etcd/client/pkg/v3/systemd"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"path/filepath"
	"time"
)

type journalWriter struct {
	io.Writer
	journalEncoder zapcore.Encoder
	consoleEncoder zapcore.Encoder
}

func newJournalWriter(wr io.Writer, encConfig zapcore.EncoderConfig) (io.Writer, error) {
	// encoder to use when writer falls back to stderr.
	consoleEncoder := zapcore.NewConsoleEncoder(encConfig)
	// TimeKey omitted for logs sent to systemd journal.
	encConfig.TimeKey = zapcore.OmitKey

	return &journalWriter{wr, zapcore.NewConsoleEncoder(encConfig), consoleEncoder}, systemd.DialJournal()
}

// priorities maps zapcore.Level to journal.Priority.
var priorities = map[zapcore.Level]journal.Priority{
	zapcore.DebugLevel:  journal.PriDebug,
	zapcore.InfoLevel:   journal.PriInfo,
	zapcore.WarnLevel:   journal.PriWarning,
	zapcore.ErrorLevel:  journal.PriErr,
	zapcore.FatalLevel:  journal.PriCrit,
	zapcore.PanicLevel:  journal.PriCrit,
	zapcore.DPanicLevel: journal.PriCrit,
}

// logLine reads the logger fields which are used to construct and send log messages to journal.
type logLine struct {
	Level      string `json:"level"`
	Time       string `json:"ts"`
	LoggerName string `json:"logger"`
	Caller     string `json:"caller"`
	Message    string `json:"msg"`
	Stack      string `json:"stacktrace"`
}

// Write converts the given byte slice into logLine before trying to send the log entry to journald.
// Should sending the data to journald fail, Write falls back to stderr for error logging.
func (w *journalWriter) Write(p []byte) (int, error) {
	line := &logLine{}
	if err := json.NewDecoder(bytes.NewReader(p)).Decode(line); err != nil {
		panic(err)
	}
	var lvl zapcore.Level
	if err := lvl.Set(line.Level); err != nil {
		panic(err)
	}
	pri, ok := priorities[lvl]
	if !ok {
		panic(fmt.Errorf("unknown log level: %q", line.Level))
	}

	entry := zapcore.Entry{
		Level:      lvl,
		LoggerName: line.LoggerName,
		Message:    line.Message,
		Stack:      line.Stack,
	}

	message, err := w.journalEncoder.EncodeEntry(entry, nil)

	if err != nil {
		panic(err)
	}

	err = journal.Send(message.String(), pri, map[string]string{
		"PACKAGE":           filepath.Dir(line.Caller),
		"SYSLOG_IDENTIFIER": filepath.Base(os.Args[0]),
	})
	if err != nil {
		// writer falls back to stderr, and timestamp needed for stderr is parsed from line.Time string.
		tm, err := time.Parse("2006-01-02T15:04:05.000-0700", line.Time)
		if err != nil {
			panic(err)
		}
		entry.Time = tm
		message, err := w.consoleEncoder.EncodeEntry(entry, nil)
		if err != nil {
			panic(err)
		}

		return w.Writer.Write(message.Bytes())
	}

	return 0, nil
}
