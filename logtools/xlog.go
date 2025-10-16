package logtools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

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
func WithFields(fields map[string]interface{}) Option {
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

// WithTimeFormat set time format string, e.g. "2006-01-02 15:04:05"
func WithTimeFormat(timeFormat string) Option {
	return func(kc *LogConfig) {
		kc.TimeFormat = timeFormat
	}
}

// WithMessageKey set message key, default is "msg"
func WithMessageKey(messageKey string) Option {
	return func(kc *LogConfig) {
		kc.MessageKey = messageKey
	}
}

func WithSource(source bool) Option {
	return func(kc *LogConfig) {
		kc.Source = source
	}
}

type Option func(c *LogConfig)

// NewXLog create NewXLog with Option, creates a new instance each time
func NewXLog(opts ...Option) *Log {
	// 创建新的配置副本，避免修改默认配置
	config := LogConfig{
		Level:   defaultKeLogConfig.Level,
		Writers: make([]io.Writer, 0),
		LogFileConfig: &LogFileConfig{
			LogFileName: defaultKeLogConfig.LogFileConfig.LogFileName,
			MaxKept:     defaultKeLogConfig.LogFileConfig.MaxKept,
			MaxMegaByte: defaultKeLogConfig.LogFileConfig.MaxMegaByte,
			MaxAge:      defaultKeLogConfig.LogFileConfig.MaxAge,
		},
		LogFormat:  defaultKeLogConfig.LogFormat,
		TimeFormat: defaultKeLogConfig.TimeFormat,
		MessageKey: defaultKeLogConfig.MessageKey,
		Fields:     make(map[string]interface{}),
		Source:     defaultKeLogConfig.Source,
	}

	logger := &Log{
		once:         sync.Once{},
		writerCloser: make([]io.WriteCloser, 0),
		logConfig:    &config,
	}

	// 应用选项
	for _, opt := range opts {
		opt(logger.logConfig)
	}

	// 如果指定了文件名，创建文件写入器
	if logger.logConfig.LogFileConfig.LogFileName != "" {
		jackLogger := &lumberjack.Logger{
			Filename:   logger.logConfig.LogFileConfig.LogFileName,
			MaxSize:    logger.logConfig.LogFileConfig.MaxMegaByte, // megabytes
			MaxBackups: logger.logConfig.LogFileConfig.MaxKept,
			MaxAge:     logger.logConfig.LogFileConfig.MaxAge, //days
			Compress:   true,                                  // disabled by default
		}

		logger.writerCloser = append(logger.writerCloser, jackLogger)
		logger.logConfig.Writers = append(logger.logConfig.Writers, jackLogger)
	}

	// 将 Fields 转换为 slog.With() 需要的参数格式
	withArgs := convertFieldsToArgs(logger.logConfig.Fields)

	// 创建handler选项，支持自定义时间格式和消息key
	handlerOptions := &slog.HandlerOptions{
		Level: switchLevel(logger.logConfig.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 处理时间格式
			if a.Key == slog.TimeKey && logger.logConfig.TimeFormat != "" {
				return slog.String(slog.TimeKey, a.Value.Time().Format(logger.logConfig.TimeFormat))
			}
			// 处理消息key
			if a.Key == slog.MessageKey && logger.logConfig.MessageKey != "" {
				return slog.String(logger.logConfig.MessageKey, a.Value.String())
			}
			return a
		},
		AddSource: logger.logConfig.Source,
	}

	var baseHandler slog.Handler
	if logger.logConfig.LogFormat == JsonFormat {
		baseHandler = slog.NewJSONHandler(io.MultiWriter(logger.logConfig.Writers...), handlerOptions)
	} else {
		baseHandler = slog.NewTextHandler(io.MultiWriter(logger.logConfig.Writers...), handlerOptions)
	}

	logger.logger = slog.New(baseHandler).With(withArgs...)

	// 设置信号处理（只为第一个创建的logger设置）
	if Logger == nil {
		Logger = logger
		go logger.once.Do(
			func() {
				logger.preStop()
			})
	}

	return logger
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
	if kl.logConfig.Source {
		kl.logWithSource(ctx, slog.LevelDebug, msg, args...)
	} else {
		kl.contextLogger(&ctx).DebugContext(ctx, msg, args...)
	}
}

// InfoWithContext record log with level [INFO] with given context for concurrency write
func (kl *Log) InfoWithContext(ctx context.Context, msg string, args ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(ctx, slog.LevelInfo, msg, args...)
	} else {
		kl.contextLogger(&ctx).InfoContext(ctx, msg, args...)
	}
}

// WarnWithContext record log with level [WARN] with given context for concurrency write
func (kl *Log) WarnWithContext(ctx context.Context, msg string, args ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(ctx, slog.LevelWarn, msg, args...)
	} else {
		kl.contextLogger(&ctx).WarnContext(ctx, msg, args...)
	}
}

// ErrorWithContext record log with level [ERROR] with given context for concurrency write
func (kl *Log) ErrorWithContext(ctx context.Context, msg string, args ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(ctx, slog.LevelError, msg, args...)
	} else {
		kl.contextLogger(&ctx).ErrorContext(ctx, msg, args...)
	}
}

// Debug record log with level [DEBUG]
func (kl *Log) Debug(msg string, args ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelDebug, msg, args...)
	} else {
		kl.logger.Debug(msg, args...)
	}
}

// Debugf record log with level [DEBUG]
func (kl *Log) Debugf(format string, a ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelDebug, fmt.Sprintf(format, a...))
	} else {
		kl.logger.Debug(fmt.Sprintf(format, a...))
	}
}

// Info record log with level [INFO]
func (kl *Log) Info(msg string, args ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelInfo, msg, args...)
	} else {
		kl.logger.Info(msg, args...)
	}
}

// Infof record log with level [INFO]
func (kl *Log) Infof(format string, a ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelInfo, fmt.Sprintf(format, a...))
	} else {
		kl.logger.Info(fmt.Sprintf(format, a...))
	}
}

// Warn record log with level [WARN]
func (kl *Log) Warn(msg string, args ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelWarn, msg, args...)
	} else {
		kl.logger.Warn(msg, args...)
	}
}

// Warnf record log with level [Warn]
func (kl *Log) Warnf(format string, a ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelWarn, fmt.Sprintf(format, a...))
	} else {
		kl.logger.Warn(fmt.Sprintf(format, a...))
	}
}

// Error record log with level [ERROR]
func (kl *Log) Error(msg string, args ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelError, msg, args...)
	} else {
		kl.logger.Error(msg, args...)
	}
}

// Errorf record log with level [Error]
func (kl *Log) Errorf(format string, a ...any) {
	if kl.logConfig.Source {
		kl.logWithSource(context.Background(), slog.LevelError, fmt.Sprintf(format, a...))
	} else {
		kl.logger.Error(fmt.Sprintf(format, a...))
	}
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
	LogFormat:  JsonFormat,
	TimeFormat: "", // 空字符串表示使用slog默认时间格式
	MessageKey: "", // 空字符串表示使用slog默认消息key "msg"
	Source:     false,
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
	Fields        map[string]interface{}
	TimeFormat    string // 时间格式化字符串，如 "2006-01-02 15:04:05"
	MessageKey    string // 消息体的key，默认为 "msg"
	Source        bool   //是否添加日志文件和行数
}

type LogFileConfig struct {
	LogFileName string
	MaxKept     int
	MaxMegaByte int
	MaxAge      int
}

// convertFieldsToArgs 将 Fields 转换为 slog.With() 需要的参数格式
func convertFieldsToArgs(fields map[string]interface{}) []any {
	var args []any
	for key, value := range fields {
		args = append(args, key, value)
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

// logWithSource 记录带有正确调用栈信息的日志
func (kl *Log) logWithSource(ctx context.Context, level slog.Level, msg string, args ...any) {
	if !kl.logger.Enabled(ctx, level) {
		return
	}

	var pc uintptr
	var pcs [1]uintptr
	// 获取调用者的程序计数器，跳过: runtime.Callers -> logWithSource -> InfoWithContext -> TestLogUtilForConcurrency
	runtime.Callers(2, pcs[:])
	pc = pcs[0]

	r := slog.NewRecord(time.Now(), level, msg, pc)
	r.Add(args...)

	// 如果有context logger，使用它；否则使用默认logger
	logger := kl.contextLogger(&ctx)
	logger.Handler().Handle(ctx, r)
}
