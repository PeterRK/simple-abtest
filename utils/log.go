package utils

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"unsafe"

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
	logger.entry = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   path,
			MaxSize:    256,
			MaxBackups: maxBackups,
			MaxAge:     maxDays,
			Compress:   true,
		}), logger.level))
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

func init() {
	if unsafe.Sizeof(zap.DebugLevel) > unsafe.Sizeof(uintptr(0)) {
		panic("zapcore.Level is not simple")
	}
}

// LogFilter provides a lightweight level filter in front of the global logger.
type LogFilter struct {
	level zapcore.Level
}

// SetLevel sets the minimum enabled level on LogFilter.
func (l *LogFilter) SetLevel(lv LogLevel) {
	l.level = zapcore.Level(lv)
}

func (l *LogFilter) Debug(args ...interface{}) {
	if l.level.Enabled(zap.DebugLevel) {
		logger.sugar.Debug(args...)
	}
}

func (l *LogFilter) Info(args ...interface{}) {
	if l.level.Enabled(zap.InfoLevel) {
		logger.sugar.Info(args...)
	}
}

func (l *LogFilter) Warn(args ...interface{}) {
	if l.level.Enabled(zap.WarnLevel) {
		logger.sugar.Warn(args...)
	}
}

func (l *LogFilter) Error(args ...interface{}) {
	if l.level.Enabled(zap.ErrorLevel) {
		logger.sugar.Error(args...)
	}
}

func (l *LogFilter) Debugf(tpl string, args ...interface{}) {
	if l.level.Enabled(zap.DebugLevel) {
		logger.sugar.Debugf(tpl, args...)
	}
}

func (l *LogFilter) Infof(tpl string, args ...interface{}) {
	if l.level.Enabled(zap.InfoLevel) {
		logger.sugar.Infof(tpl, args...)
	}
}

func (l *LogFilter) Warnf(tpl string, args ...interface{}) {
	if l.level.Enabled(zap.WarnLevel) {
		logger.sugar.Warnf(tpl, args...)
	}
}

func (l *LogFilter) Errorf(tpl string, args ...interface{}) {
	if l.level.Enabled(zap.ErrorLevel) {
		logger.sugar.Errorf(tpl, args...)
	}
}

var logTraceNum uint64

type LogCtx struct {
	trace uint64
}

func InitLogCtx() LogCtx {
	return LogCtx{
		trace: atomic.AddUint64(&logTraceNum, 1),
	}
}

func (c *LogCtx) LogDebug(msg string) {
	logger.sugar.Debugw(msg, "trace", c.trace)
}

func (c *LogCtx) LogInfo(msg string) {
	logger.sugar.Infow(msg, "trace", c.trace)
}

func (c *LogCtx) LogWarn(msg string) {
	logger.sugar.Warnw(msg, "trace", c.trace)
}

func (c *LogCtx) LogError(msg string) {
	logger.sugar.Errorw(msg, "trace", c.trace)
}

func (c *LogCtx) LogDebugf(tpl string, args ...any) {
	if logger.level.Enabled(zap.DebugLevel) {
		logger.sugar.Debugw(fmt.Sprintf(tpl, args...), "trace", c.trace)
	}
}

func (c *LogCtx) LogInfof(tpl string, args ...any) {
	if logger.level.Enabled(zap.InfoLevel) {
		logger.sugar.Infow(fmt.Sprintf(tpl, args...), "trace", c.trace)
	}
}

func (c *LogCtx) LogWarnf(tpl string, args ...any) {
	if logger.level.Enabled(zap.WarnLevel) {
		logger.sugar.Warnw(fmt.Sprintf(tpl, args...), "trace", c.trace)
	}
}

func (c *LogCtx) LogErrorf(tpl string, args ...any) {
	if logger.level.Enabled(zap.ErrorLevel) {
		logger.sugar.Errorw(fmt.Sprintf(tpl, args...), "trace", c.trace)
	}
}
