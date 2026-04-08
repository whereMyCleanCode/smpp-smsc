package smsc

import (
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogLevel int

const (
	TraceLevel LogLevel = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	DisableLevel
)

func (l LogLevel) String() string {
	switch l {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	default:
		return "info"
	}
}

func ParseLogLevel(level string) LogLevel {
	switch level {
	case "trace":
		return TraceLevel
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "fatal":
		return FatalLevel
	default:
		return InfoLevel
	}
}

type LoggerOptions struct {
	Pretty     bool
	Color      bool
	TimeFormat string
}

func DetectLoggerOptions(output io.Writer) LoggerOptions {
	tty := isTerminalWriter(output)
	return LoggerOptions{
		Pretty:     tty,
		Color:      tty,
		TimeFormat: time.RFC3339,
	}
}

func isTerminalWriter(output io.Writer) bool {
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

type LogEvent interface {
	Str(key, val string) LogEvent
	Int(key string, val int) LogEvent
	Int64(key string, val int64) LogEvent
	Uint32(key string, val uint32) LogEvent
	Uint64(key string, val uint64) LogEvent
	Uint8(key string, val uint8) LogEvent
	Bool(key string, val bool) LogEvent
	Dur(key string, val time.Duration) LogEvent
	Time(key string, val time.Time) LogEvent
	Err(err error) LogEvent
	Any(key string, val interface{}) LogEvent
	Bytes(key string, val []byte) LogEvent
	Enabled() bool
	Msg(msg string)
}

type Logger interface {
	Trace() LogEvent
	Debug() LogEvent
	Info() LogEvent
	Warn() LogEvent
	Error() LogEvent
	Fatal() LogEvent
	WithStr(key, val string) Logger
	With() LogBuilder
}

type LogBuilder interface {
	Str(key, val string) LogBuilder
	Int(key string, val int) LogBuilder
	Logger() Logger
}

type zapEvent struct {
	logger *zap.Logger
	level  zapcore.Level
	fields []zap.Field
}

func (e *zapEvent) Str(key, val string) LogEvent {
	e.fields = append(e.fields, zap.String(key, val))
	return e
}

func (e *zapEvent) Int(key string, val int) LogEvent {
	e.fields = append(e.fields, zap.Int(key, val))
	return e
}

func (e *zapEvent) Int64(key string, val int64) LogEvent {
	e.fields = append(e.fields, zap.Int64(key, val))
	return e
}

func (e *zapEvent) Uint32(key string, val uint32) LogEvent {
	e.fields = append(e.fields, zap.Uint32(key, val))
	return e
}

func (e *zapEvent) Uint64(key string, val uint64) LogEvent {
	e.fields = append(e.fields, zap.Uint64(key, val))
	return e
}

func (e *zapEvent) Uint8(key string, val uint8) LogEvent {
	e.fields = append(e.fields, zap.Uint8(key, val))
	return e
}

func (e *zapEvent) Bool(key string, val bool) LogEvent {
	e.fields = append(e.fields, zap.Bool(key, val))
	return e
}

func (e *zapEvent) Dur(key string, val time.Duration) LogEvent {
	e.fields = append(e.fields, zap.Duration(key, val))
	return e
}

func (e *zapEvent) Time(key string, val time.Time) LogEvent {
	e.fields = append(e.fields, zap.Time(key, val))
	return e
}

func (e *zapEvent) Err(err error) LogEvent {
	e.fields = append(e.fields, zap.Error(err))
	return e
}

func (e *zapEvent) Any(key string, val interface{}) LogEvent {
	e.fields = append(e.fields, zap.Any(key, val))
	return e
}

func (e *zapEvent) Bytes(key string, val []byte) LogEvent {
	e.fields = append(e.fields, zap.Binary(key, val))
	return e
}

func (e *zapEvent) Enabled() bool {
	return e.logger.Core().Enabled(e.level)
}

func (e *zapEvent) Msg(msg string) {
	switch e.level {
	case zapcore.DebugLevel:
		e.logger.Debug(msg, e.fields...)
	case zapcore.InfoLevel:
		e.logger.Info(msg, e.fields...)
	case zapcore.WarnLevel:
		e.logger.Warn(msg, e.fields...)
	case zapcore.ErrorLevel:
		e.logger.Error(msg, e.fields...)
	case zapcore.FatalLevel:
		e.logger.Fatal(msg, e.fields...)
	default:
		e.logger.Debug(msg, e.fields...)
	}
}

type zapBuilder struct {
	base   *zap.Logger
	fields []zap.Field
}

func (b *zapBuilder) Str(key, val string) LogBuilder {
	b.fields = append(b.fields, zap.String(key, val))
	return b
}

func (b *zapBuilder) Int(key string, val int) LogBuilder {
	b.fields = append(b.fields, zap.Int(key, val))
	return b
}

func (b *zapBuilder) Logger() Logger {
	return &zapLogger{
		logger: b.base.With(b.fields...),
	}
}

type zapLogger struct {
	logger *zap.Logger
	pretty bool
	color  bool
}

func (l *zapLogger) Trace() LogEvent { return &zapEvent{logger: l.logger, level: zapcore.DebugLevel} }
func (l *zapLogger) Debug() LogEvent { return &zapEvent{logger: l.logger, level: zapcore.DebugLevel} }
func (l *zapLogger) Info() LogEvent  { return &zapEvent{logger: l.logger, level: zapcore.InfoLevel} }
func (l *zapLogger) Warn() LogEvent  { return &zapEvent{logger: l.logger, level: zapcore.WarnLevel} }
func (l *zapLogger) Error() LogEvent { return &zapEvent{logger: l.logger, level: zapcore.ErrorLevel} }
func (l *zapLogger) Fatal() LogEvent { return &zapEvent{logger: l.logger, level: zapcore.FatalLevel} }

func (l *zapLogger) WithStr(key, val string) Logger {
	return &zapLogger{
		logger: l.logger.With(zap.String(key, val)),
		pretty: l.pretty,
		color:  l.color,
	}
}

func (l *zapLogger) With() LogBuilder {
	return &zapBuilder{base: l.logger}
}

func NewLogger(output io.Writer, level LogLevel) Logger {
	return NewLoggerWithOptions(output, level, DetectLoggerOptions(output))
}

func NewLoggerWithOptions(output io.Writer, level LogLevel, opts LoggerOptions) Logger {
	if opts.TimeFormat == "" {
		opts.TimeFormat = time.RFC3339
	}

	var zl zapcore.Level
	switch level {
	case TraceLevel:
		zl = zapcore.DebugLevel
	case DebugLevel:
		zl = zapcore.DebugLevel
	case InfoLevel:
		zl = zapcore.InfoLevel
	case WarnLevel:
		zl = zapcore.WarnLevel
	case ErrorLevel:
		zl = zapcore.ErrorLevel
	case FatalLevel:
		zl = zapcore.FatalLevel
	case DisableLevel:
		zl = zapcore.FatalLevel + 1
	default:
		zl = zapcore.InfoLevel
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if opts.Pretty {
		encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimeFormat)
		if opts.Color {
			encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		} else {
			encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		}
	} else {
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	}

	encoder := zapcore.NewJSONEncoder(encoderCfg)
	if opts.Pretty {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}
	core := zapcore.NewCore(encoder, zapcore.AddSync(output), zl)
	logger := zap.New(core, zap.AddCallerSkip(1))

	return &zapLogger{
		logger: logger,
		pretty: opts.Pretty,
		color:  opts.Color,
	}
}

func DefaultLogger() Logger {
	return NewLoggerWithOptions(os.Stdout, InfoLevel, DetectLoggerOptions(os.Stdout))
}
