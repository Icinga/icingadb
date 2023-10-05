package logging

import (
	"github.com/icinga/icingadb/pkg/strcase"
	"github.com/pkg/errors"
	"github.com/ssgreg/journald"
	"go.uber.org/zap/zapcore"
	"strings"
)

// priorities maps zapcore.Level to journal.Priority.
var priorities = map[zapcore.Level]journald.Priority{
	zapcore.DebugLevel:  journald.PriorityDebug,
	zapcore.InfoLevel:   journald.PriorityInfo,
	zapcore.WarnLevel:   journald.PriorityWarning,
	zapcore.ErrorLevel:  journald.PriorityErr,
	zapcore.FatalLevel:  journald.PriorityCrit,
	zapcore.PanicLevel:  journald.PriorityCrit,
	zapcore.DPanicLevel: journald.PriorityCrit,
}

// NewJournaldCore returns a zapcore.Core that sends log entries to systemd-journald and
// uses the given identifier as a prefix for structured logging context that is sent as journal fields.
func NewJournaldCore(identifier string, enab zapcore.LevelEnabler) zapcore.Core {
	return &journaldCore{
		LevelEnabler: enab,
		identifier:   identifier,
		identifierU:  strings.ToUpper(identifier),
	}
}

type journaldCore struct {
	zapcore.LevelEnabler
	context     []zapcore.Field
	identifier  string
	identifierU string
}

func (c *journaldCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}

	return ce
}

func (c *journaldCore) Sync() error {
	return nil
}

func (c *journaldCore) With(fields []zapcore.Field) zapcore.Core {
	cc := *c
	cc.context = append(cc.context[:len(cc.context):len(cc.context)], fields...)

	return &cc
}

func (c *journaldCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	pri, ok := priorities[ent.Level]
	if !ok {
		return errors.Errorf("unknown log level %q", ent.Level)
	}

	enc := zapcore.NewMapObjectEncoder()
	c.addFields(enc, fields)
	c.addFields(enc, c.context)
	enc.Fields["SYSLOG_IDENTIFIER"] = c.identifier

	message := ent.Message
	if ent.LoggerName != c.identifier {
		message = ent.LoggerName + ": " + message
	}

	return journald.Send(message, pri, enc.Fields)
}

func (c *journaldCore) addFields(enc zapcore.ObjectEncoder, fields []zapcore.Field) {
	for _, field := range fields {
		field.Key = c.identifierU +
			"_" +
			strcase.ScreamingSnake(field.Key)
		field.AddTo(enc)
	}
}
