package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String 返回日志级别字符串
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel 解析日志级别
func ParseLevel(s string) (Level, error) {
	switch s {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level: %s", s)
	}
}

// Logger 日志器
type Logger struct {
	level  Level
	logger *slog.Logger
	file   *os.File
}

// Config 日志配置
type Config struct {
	Level    string `yaml:"level"`
	Output   string `yaml:"output"`     // stdout, stderr, file
	FilePath string `yaml:"file_path"`  // 日志文件路径
}

// New 创建日志器
func New(cfg Config) (*Logger, error) {
	level, err := ParseLevel(cfg.Level)
	if err != nil {
		level = LevelInfo
	}

	var output io.Writer
	var file *os.File

	switch cfg.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	case "file":
		if cfg.FilePath == "" {
			cfg.FilePath = "logs/harness.log"
		}

		// 创建日志目录
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// 打开日志文件
		file, err = os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		output = file
	default:
		output = os.Stdout
	}

	// 创建 slog logger
	var slogLevel slog.Level
	switch level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: slogLevel,
	}))

	return &Logger{
		level:  level,
		logger: logger,
		file:   file,
	}, nil
}

// Close 关闭日志器
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Debug 调试日志
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info 信息日志
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn 警告日志
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error 错误日志
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Fatal 致命日志
func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Error(msg, args...)
	os.Exit(1)
}

// With 添加字段
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		level:  l.level,
		logger: l.logger.With(args...),
		file:   l.file,
	}
}

// 全局日志器
var defaultLogger *Logger

func init() {
	var err error
	defaultLogger, err = New(Config{
		Level:  "info",
		Output: "stdout",
	})
	if err != nil {
		slog.Error("failed to create default logger", "error", err)
	}
}

// SetDefault 设置默认日志器
func SetDefault(logger *Logger) {
	defaultLogger = logger
}

// GetDefault 获取默认日志器
func GetDefault() *Logger {
	return defaultLogger
}

// Debug 调试日志
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// Info 信息日志
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// Warn 警告日志
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

// Error 错误日志
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// Fatal 致命日志
func Fatal(msg string, args ...any) {
	defaultLogger.Fatal(msg, args...)
}

// With 添加字段
func With(args ...any) *Logger {
	return defaultLogger.With(args...)
}
