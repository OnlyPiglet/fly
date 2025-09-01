package logtools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/natefinch/lumberjack"
)

const (
	JsonFormat = iota
	TextFormat
)

const (
	DEBUG = iota
	INFO
	WARN
	ERROR
)

type LoggerContextKey struct{}

// GetLogger return instance of [Logger].
func GetLogger(ctx context.Context) *Log {
	return Logger
}

// WithLevel set log level
func WithLevel(level int) Option {
	return func(kc *LogConfig) {
		kc.Level = level
	}
}

// WithFileName set log file full path, if not set or empty,will not generate file type log
func WithFileName(filename string) Option {
	return func(kc *LogConfig) {
		if kc.LogFileConfig == nil {
			kc.LogFileConfig = &LogFileConfig{}
		}
		kc.LogFileConfig.LogFileName = filename
	}
}

// WithFields 带上可以复用的上下文字段，key 为 string,value 是 基本类型如 int64,float64,string
func WithFields(fields []map[string]interface{}) Option {
	return func(kc *LogConfig) {
		kc.Fields = fields
	}
}

// WithMaxMegaSize set single log file max mb size
func WithMaxMegaSize(maxMegaByte int) Option {
	return func(kc *LogConfig) {
		kc.LogFileConfig.MaxMegaByte = maxMegaByte
	}
}

// WithMaxKept set max keep log file count
func WithMaxKept(maxKept int) Option {
	return func(kc *LogConfig) {
		kc.LogFileConfig.MaxKept = maxKept
	}
}

// WithAgeKept set max keep log file count
func WithAgeKept(maxAge int) Option {
	return func(kc *LogConfig) {
		kc.LogFileConfig.MaxAge = maxAge
	}
}

// WithWriters set other log writer
func WithWriters(writers ...io.Writer) Option {
	return func(kc *LogConfig) {
		kc.Writers = writers
	}
}

// WithLogFormat set log format with [ TextFormat , JsonFormat]
func WithLogFormat(logFormat int) Option {
	return func(kc *LogConfig) {
		kc.LogFormat = logFormat
	}
}

type Option func(c *LogConfig)

// NewXLog create NewXLog with Option,may has race create,but don't be care
func NewXLog(opts ...Option) *Log {

	if Logger != nil {
		return Logger
	}

	Logger = &Log{
		once:         sync.Once{},
		writerCloser: make([]io.WriteCloser, 0),
		logConfig:    &defaultKeLogConfig,
	}

	defer func() {
		go Logger.once.Do(
			func() {
				Logger.preStop()
			})
	}()

	for _, opt := range opts {
		opt(Logger.logConfig)
	}

	if Logger.logConfig.LogFileConfig.LogFileName != "" {

		jackLogger := &lumberjack.Logger{
			Filename:   Logger.logConfig.LogFileConfig.LogFileName,
			MaxSize:    Logger.logConfig.LogFileConfig.MaxMegaByte, // megabytes
			MaxBackups: Logger.logConfig.LogFileConfig.MaxKept,
			MaxAge:     Logger.logConfig.LogFileConfig.MaxAge, //days
			Compress:   true,                                  // disabled by default
		}

		Logger.writerCloser = append(Logger.writerCloser, jackLogger)
		Logger.logConfig.Writers = append(Logger.logConfig.Writers, jackLogger)

	}

	// 将 Fields 转换为 slog.With() 需要的参数格式
	withArgs := convertFieldsToArgs(Logger.logConfig.Fields)

	if Logger.logConfig.LogFormat == JsonFormat {
		Logger.logger = slog.New(slog.NewJSONHandler(io.MultiWriter(Logger.logConfig.Writers...), &slog.HandlerOptions{Level: switchLevel(Logger.logConfig.Level)})).With(withArgs...)
	} else {
		Logger.logger = slog.New(slog.NewTextHandler(io.MultiWriter(Logger.logConfig.Writers...), &slog.HandlerOptions{Level: switchLevel(Logger.logConfig.Level)})).With(withArgs...)
	}
	return Logger
}

// Close recycle resource before program exit like exit(0)
func (kl *Log) Close() {
	for _, closer := range kl.writerCloser {
		closer.Close()
	}
}

// AddAttrWithContext add key/value attr pairs on child logger of KeLog.logger for concurrency write
func (kl *Log) AddAttrWithContext(ctx context.Context, args ...any) context.Context {
	logger := kl.contextLogger(&ctx).With(args...)
	updateContextLogger(&ctx, logger)
	return ctx
}

// DebugWithContext record log with level [DEBUG] with given context for concurrency write
func (kl *Log) DebugWithContext(ctx context.Context, msg string, args ...any) {
	kl.contextLogger(&ctx).DebugContext(ctx, msg, args...)
}

// InfoWithContext record log with level [INFO] with given context for concurrency write
func (kl *Log) InfoWithContext(ctx context.Context, msg string, args ...any) {
	kl.contextLogger(&ctx).InfoContext(ctx, msg, args...)
}

// WarnWithContext record log with level [WARN] with given context for concurrency write
func (kl *Log) WarnWithContext(ctx context.Context, msg string, args ...any) {
	kl.contextLogger(&ctx).WarnContext(ctx, msg, args...)
}

// ErrorWithContext record log with level [ERROR] with given context for concurrency write
func (kl *Log) ErrorWithContext(ctx context.Context, msg string, args ...any) {
	kl.contextLogger(&ctx).ErrorContext(ctx, msg, args...)
}

// Debug record log with level [DEBUG]
func (kl *Log) Debug(msg string, args ...any) {
	kl.logger.Debug(msg, args...)
}

// Debugf record log with level [DEBUG]
func (kl *Log) Debugf(format string, a ...any) {
	kl.logger.Debug(fmt.Sprintf(format, a...))
}

// Info record log with level [INFO]
func (kl *Log) Info(msg string, args ...any) {
	kl.logger.Info(msg, args...)
}

// Infof record log with level [INFO]
func (kl *Log) Infof(format string, a ...any) {
	kl.logger.Info(fmt.Sprintf(format, a...))
}

// Warn record log with level [WARN]
func (kl *Log) Warn(msg string, args ...any) {
	kl.logger.Warn(msg, args...)
}

// Warnf record log with level [Warn]
func (kl *Log) Warnf(format string, a ...any) {
	kl.logger.Warn(fmt.Sprintf(format, a...))
}

// Error record log with level [ERROR]
func (kl *Log) Error(msg string, args ...any) {
	kl.logger.Error(msg, args...)
}

// Errorf record log with level [Error]
func (kl *Log) Errorf(format string, a ...any) {
	kl.logger.Error(fmt.Sprintf(format, a...))
}

// AddAttrs add key/value attr pairs on KeLog.logger
func (kl *Log) AddAttrs(args ...any) {
	kl.logger = kl.logger.With(args...)
}

var defaultKeLogConfig = LogConfig{
	Level:   ERROR,
	Writers: []io.Writer{os.Stdout},
	LogFileConfig: &LogFileConfig{
		LogFileName: "",
		MaxKept:     7,
		MaxMegaByte: 100,
		MaxAge:      28,
	},
	LogFormat: JsonFormat,
}

var Logger *Log

type Log struct {
	logConfig    *LogConfig
	writerCloser []io.WriteCloser
	logger       *slog.Logger
	once         sync.Once
}

type LogConfig struct {
	Level         int
	Writers       []io.Writer
	LogFileConfig *LogFileConfig
	LogFormat     int
	Fields        []map[string]interface{}
}

type LogFileConfig struct {
	LogFileName string
	MaxKept     int
	MaxMegaByte int
	MaxAge      int
}

// convertFieldsToArgs 将 Fields 转换为 slog.With() 需要的参数格式
func convertFieldsToArgs(fields []map[string]interface{}) []any {
	var args []any
	for _, fieldMap := range fields {
		for key, value := range fieldMap {
			args = append(args, key, value)
		}
	}
	return args
}

func switchLevel(level int) slog.Level {
	switch level {
	case DEBUG:
		return slog.LevelDebug
	case INFO:
		return slog.LevelInfo
	case WARN:
		return slog.LevelWarn
	case ERROR:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}

func (kl *Log) close() {
	for _, closer := range kl.writerCloser {
		closer.Close()
	}
}

func (kl *Log) preStop() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for sig := range c {
		signal.Stop(c)
		kl.close()
		p, err := os.FindProcess(syscall.Getpid())
		if err != nil {
			fmt.Println(err)
			os.Exit(0)
		}
		if err := p.Signal(sig); err != nil {
			fmt.Println(err)
		}
	}
}

// contextLogger get Logger by child logger of KeLog.logger for concurrency write
func (kl *Log) contextLogger(ctx *context.Context) *slog.Logger {
	logger, ok := (*ctx).Value(LoggerContextKey{}).(*slog.Logger)
	if !ok {
		logger = kl.logger.With(slog.Group(""))
		updateContextLogger(ctx, logger)
	}
	return logger
}

// updateContextLogger update new logger with some option
func updateContextLogger(ctx *context.Context, logger *slog.Logger) {
	*ctx = context.WithValue(*ctx, LoggerContextKey{}, logger)
}
