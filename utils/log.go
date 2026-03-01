package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger struct {
	level zap.AtomicLevel
	entry *zap.Logger
	sugar *zap.SugaredLogger
}

// GetRawLogger returns the underlying zap.Logger instance.
func GetRawLogger() *zap.Logger {
	return logger.entry
}

// GetLogger returns the global sugared logger.
func GetLogger() *zap.SugaredLogger {
	return logger.sugar
}

// HttpChangeLogLevel changes the global log level based on the request body.
// It accepts DEBUG/INFO/WARN/ERROR (case-insensitive).
func HttpChangeLogLevel(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch string(raw) {
	case "DEBUG", "debug":
		SetLogLevel(DebugLevel)
	case "INFO", "info":
		SetLogLevel(InfoLevel)
	case "WARN", "warn":
		SetLogLevel(WarnLevel)
	case "ERROR", "error":
		SetLogLevel(ErrorLevel)
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

// InitLog initializes the global logger writing JSON logs to the given path.
// Repeated calls after initialization are ignored.
func InitLog(path string, maxBackups, maxDays int) {
	if logger.entry != nil {
		return
	}
	logger.level = zap.NewAtomicLevel()
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder

	var writer io.Writer = os.Stderr
	if len(path) != 0 {
		writer = &lumberjack.Logger{
			Filename:   path,
			MaxSize:    256,
			MaxBackups: maxBackups,
			MaxAge:     maxDays,
			Compress:   true,
		}
	}

	logger.entry = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg),
		zapcore.AddSync(writer), logger.level))
	logger.sugar = logger.entry.Sugar()
}

// SyncLog flushes buffered log entries to the underlying writer.
func SyncLog() {
	if logger.entry != nil {
		logger.entry.Sync()
	}
}

// LogLevel is an alias of zapcore.Level for log level configuration.
type LogLevel = zapcore.Level

const (
	DebugLevel = zap.DebugLevel
	InfoLevel  = zap.InfoLevel
	WarnLevel  = zap.WarnLevel
	ErrorLevel = zap.ErrorLevel
	FatalLevel = zap.FatalLevel
)

// LogLevelEnabled reports whether the given level is enabled on the global logger.
func LogLevelEnabled(lv LogLevel) bool {
	return logger.level.Enabled(lv)
}

// SetLogLevel updates the global logger level at runtime.
func SetLogLevel(lv LogLevel) {
	logger.level.SetLevel(lv)
}

var logTraceNum uint64

// ContextLogger logs with a unique trace id and optional context label.
type ContextLogger struct {
	id  uint64
	ctx string
}

// NewContextLogger returns a ContextLogger with the provided context label.
func NewContextLogger(ctx string) *ContextLogger {
	return &ContextLogger{
		id:  atomic.AddUint64(&logTraceNum, 1),
		ctx: ctx,
	}
}

// Debug logs a debug message with the context trace fields.
func (l *ContextLogger) Debug(msg string) {
	if len(l.ctx) == 0 {
		logger.sugar.Debugw(msg, "id", l.id)
	} else {
		logger.sugar.Debugw(msg, "id", l.id, "ctx", l.ctx)
	}
}

// Info logs an info message with the context trace fields.
func (l *ContextLogger) Info(msg string) {
	if len(l.ctx) == 0 {
		logger.sugar.Infow(msg, "id", l.id)
	} else {
		logger.sugar.Infow(msg, "id", l.id, "ctx", l.ctx)
	}
}

// Warn logs a warning message with the context trace fields.
func (l *ContextLogger) Warn(msg string) {
	if len(l.ctx) == 0 {
		logger.sugar.Warnw(msg, "id", l.id)
	} else {
		logger.sugar.Warnw(msg, "id", l.id, "ctx", l.ctx)
	}
}

// Error logs an error message with the context trace fields.
func (l *ContextLogger) Error(msg string) {
	if len(l.ctx) == 0 {
		logger.sugar.Errorw(msg, "id", l.id)
	} else {
		logger.sugar.Errorw(msg, "id", l.id, "ctx", l.ctx)
	}
}

// Debugf formats and logs a debug message when debug level is enabled.
func (l *ContextLogger) Debugf(tpl string, args ...any) {
	if logger.level.Enabled(zap.DebugLevel) {
		l.Debug(fmt.Sprintf(tpl, args...))
	}
}

// Infof formats and logs an info message when info level is enabled.
func (l *ContextLogger) Infof(tpl string, args ...any) {
	if logger.level.Enabled(zap.InfoLevel) {
		l.Info(fmt.Sprintf(tpl, args...))
	}
}

// Warnf formats and logs a warning message when warn level is enabled.
func (l *ContextLogger) Warnf(tpl string, args ...any) {
	if logger.level.Enabled(zap.WarnLevel) {
		l.Warn(fmt.Sprintf(tpl, args...))
	}
}

// Errorf formats and logs an error message when error level is enabled.
func (l *ContextLogger) Errorf(tpl string, args ...any) {
	if logger.level.Enabled(zap.ErrorLevel) {
		l.Error(fmt.Sprintf(tpl, args...))
	}
}
