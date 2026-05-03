package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateID 生成唯一 ID
func GenerateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// GenerateUUID 生成 UUID
func GenerateUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b), nil
}

// Contains 检查切片是否包含元素
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Unique 去重
func Unique(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// Map 映射切片
func Map(slice []string, fn func(string) string) []string {
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = fn(s)
	}
	return result
}

// Filter 过滤切片
func Filter(slice []string, fn func(string) bool) []string {
	result := []string{}
	for _, s := range slice {
		if fn(s) {
			result = append(result, s)
		}
	}
	return result
}

// Reduce 归约切片
func Reduce(slice []string, fn func(string, string) string, initial string) string {
	result := initial
	for _, s := range slice {
		result = fn(result, s)
	}
	return result
}

// Chunk 分块切片
func Chunk(slice []string, size int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// Flatten 展平切片
func Flatten(slices [][]string) []string {
	var result []string
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}

// Reverse 反转切片
func Reverse(slice []string) []string {
	result := make([]string, len(slice))
	for i, s := range slice {
		result[len(slice)-1-i] = s
	}
	return result
}

// FormatDuration 格式化持续时间
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", float64(d)/float64(time.Second))
	}
	if d < time.Hour {
		return fmt.Sprintf("%.2fm", float64(d)/float64(time.Minute))
	}
	return fmt.Sprintf("%.2fh", float64(d)/float64(time.Hour))
}

// FormatSize 格式化大小
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatTime 格式化时间
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// ParseTime 解析时间
func ParseTime(s string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", s)
}

// MinInt 返回最小整数
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxInt 返回最大整数
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinFloat64 返回最小浮点数
func MinFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// MaxFloat64 返回最大浮点数
func MaxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ClampInt 将整数限制在范围内
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampFloat64 将浮点数限制在范围内
func ClampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
