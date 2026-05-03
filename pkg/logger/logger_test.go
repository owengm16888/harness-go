package logger

import (
	"testing"
)

func TestLogger_Debug(t *testing.T) {
	logger, _ := New(Config{
		Level:  "debug",
		Output: "stdout",
	})

	// 测试不会 panic
	logger.Debug("test message")
	logger.Debug("test message with args: %s %d", "hello", 123)
}

func TestLogger_Info(t *testing.T) {
	logger, _ := New(Config{
		Level:  "info",
		Output: "stdout",
	})

	logger.Info("test message")
	logger.Info("test message with args: %s %d", "hello", 123)
}

func TestLogger_Warn(t *testing.T) {
	logger, _ := New(Config{
		Level:  "warn",
		Output: "stdout",
	})

	logger.Warn("test message")
	logger.Warn("test message with args: %s %d", "hello", 123)
}

func TestLogger_Error(t *testing.T) {
	logger, _ := New(Config{
		Level:  "error",
		Output: "stdout",
	})

	logger.Error("test message")
	logger.Error("test message with args: %s %d", "hello", 123)
}

func TestLogger_With(t *testing.T) {
	logger, _ := New(Config{
		Level:  "info",
		Output: "stdout",
	})

	logger.With("key", "value").Info("test message")
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"debug", LevelDebug, false},
		{"info", LevelInfo, false},
		{"warn", LevelWarn, false},
		{"error", LevelError, false},
		{"fatal", LevelFatal, false},
		{"invalid", LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLevel(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if level != tt.expected {
				t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, level, tt.expected)
			}
		})
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("Level.String() = %s, want %s", tt.level.String(), tt.expected)
			}
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	// 测试默认日志器
	Debug("test debug")
	Info("test info")
	Warn("test warn")
	Error("test error")

	// 测试全局字段
	With("key", "value").Info("test with field")
}

func TestNewFromFile(t *testing.T) {
	// 测试文件输出
	logger, err := New(Config{
		Level:    "info",
		Output:   "file",
		FilePath: "/tmp/test-harness.log",
	})

	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("test message to file")
}
