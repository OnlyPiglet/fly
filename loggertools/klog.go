package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
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

// WithFileLogAutoFlushInterval set file log auto flush interval,if 0 will close auto flush
func WithFileLogAutoFlushInterval(interval time.Duration) Option {
	return func(kc *LogConfig) {
		if kc.LogFileConfig == nil {
			kc.LogFileConfig = &LogFileConfig{}
		}
		kc.LogFileConfig.FileLogAutoFlushInterval = interval
	}
}

// WithLogFileName set log file full path, if not set or empty,will not generate file type log
func WithLogFileName(filename string) Option {
	return func(kc *LogConfig) {
		if kc.LogFileConfig == nil {
			kc.LogFileConfig = &LogFileConfig{}
		}
		kc.LogFileConfig.LogFileName = filename
	}
}

// WithLogFileNameMaxSize set single log file max size
func WithLogFileNameMaxSize(maxSize int64) Option {
	return func(kc *LogConfig) {
		kc.LogFileConfig.MaxSize = maxSize
	}
}

// WithLogFileNameMaxKept set max keep log file count
func WithLogFileNameMaxKept(maxKept int64) Option {
	return func(kc *LogConfig) {
		kc.LogFileConfig.MaxKept = maxKept
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

// NewKeLog create KeLog with Option,may has race create,but don't be care
func NewKeLog(opts ...Option) *Log {

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
		writer := NewFileWriter(Logger.logConfig.LogFileConfig.LogFileName, NewFileRotator(Logger.logConfig.LogFileConfig.MaxSize, Logger.logConfig.LogFileConfig.MaxKept))
		Logger.writerCloser = append(Logger.writerCloser, writer)
		Logger.logConfig.Writers = append(Logger.logConfig.Writers, writer)

		// start auto flush of file logger
		if Logger.logConfig.LogFileConfig.FileLogAutoFlushInterval != 0 {
			go func() {
				timer := time.NewTicker(Logger.logConfig.LogFileConfig.FileLogAutoFlushInterval)
				fileWriter, ok := writer.(*FileWriter)
				if ok {
					for range timer.C {
						fileWriter.flush()
					}
				}
			}()
		}

	}

	if Logger.logConfig.LogFormat == JsonFormat {
		Logger.logger = slog.New(slog.NewJSONHandler(io.MultiWriter(Logger.logConfig.Writers...), &slog.HandlerOptions{Level: switchLevel(Logger.logConfig.Level)}))
	} else {
		Logger.logger = slog.New(slog.NewTextHandler(io.MultiWriter(Logger.logConfig.Writers...), &slog.HandlerOptions{Level: switchLevel(Logger.logConfig.Level)}))
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

// Info record log with level [INFO]
func (kl *Log) Info(msg string, args ...any) {
	kl.logger.Info(msg, args...)
}

// Warn record log with level [WARN]
func (kl *Log) Warn(msg string, args ...any) {
	kl.logger.Warn(msg, args...)
}

// Error record log with level [ERROR]
func (kl *Log) Error(msg string, args ...any) {
	kl.logger.Error(msg, args...)
}

// AddAttrs add key/value attr pairs on KeLog.logger
func (kl *Log) AddAttrs(args ...any) {
	kl.logger = kl.logger.With(args...)
}

var defaultKeLogConfig = LogConfig{
	Level:   ERROR,
	Writers: []io.Writer{os.Stdout},
	LogFileConfig: &LogFileConfig{
		LogFileName:              "",
		MaxKept:                  7,
		MaxSize:                  DefaultFileRotator.MaxSize(),
		FileLogAutoFlushInterval: 1 * time.Second,
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
}

type LogFileConfig struct {
	LogFileName              string
	MaxKept                  int64
	MaxSize                  int64
	FileLogAutoFlushInterval time.Duration
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
