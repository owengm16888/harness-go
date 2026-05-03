package utils

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}

	if id1 == id2 {
		t.Error("Expected unique IDs")
	}
}

func TestGenerateUUID(t *testing.T) {
	uuid1, err := GenerateUUID()
	if err != nil {
		t.Fatalf("Failed to generate UUID: %v", err)
	}

	uuid2, err := GenerateUUID()
	if err != nil {
		t.Fatalf("Failed to generate UUID: %v", err)
	}

	if uuid1 == "" {
		t.Error("Expected non-empty UUID")
	}

	if uuid1 == uuid2 {
		t.Error("Expected unique UUIDs")
	}

	// UUID 格式: 8-4-4-4-12
	if len(uuid1) < 32 {
		t.Errorf("Expected UUID length 36, got %d", len(uuid1))
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "a", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{nil, "a", false},
	}

	for _, tt := range tests {
		result := Contains(tt.slice, tt.item)
		if result != tt.expected {
			t.Errorf("Contains(%v, %s) = %v, want %v", tt.slice, tt.item, result, tt.expected)
		}
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		slice    []string
		expected []string
	}{
		{[]string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{}, []string{}},
		{nil, []string{}},
	}

	for _, tt := range tests {
		result := Unique(tt.slice)
		if len(result) != len(tt.expected) {
			t.Errorf("Unique(%v) length = %d, want %d", tt.slice, len(result), len(tt.expected))
		}
	}
}

func TestMap(t *testing.T) {
	slice := []string{"a", "b", "c"}
	result := Map(slice, func(s string) string {
		return s + "!"
	})

	expected := []string{"a!", "b!", "c!"}
	if len(result) != len(expected) {
		t.Errorf("Map result length = %d, want %d", len(result), len(expected))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("Map[%d] = %s, want %s", i, v, expected[i])
		}
	}
}

func TestFilter(t *testing.T) {
	slice := []string{"a", "b", "c", "d"}
	result := Filter(slice, func(s string) bool {
		return s == "a" || s == "c"
	})

	expected := []string{"a", "c"}
	if len(result) != len(expected) {
		t.Errorf("Filter result length = %d, want %d", len(result), len(expected))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("Filter[%d] = %s, want %s", i, v, expected[i])
		}
	}
}

func TestReduce(t *testing.T) {
	slice := []string{"a", "b", "c"}
	result := Reduce(slice, func(acc, s string) string {
		return acc + s
	}, "")

	if result != "abc" {
		t.Errorf("Reduce = %s, want 'abc'", result)
	}
}

func TestChunk(t *testing.T) {
	slice := []string{"a", "b", "c", "d", "e"}
	result := Chunk(slice, 2)

	expected := [][]string{
		{"a", "b"},
		{"c", "d"},
		{"e"},
	}

	if len(result) != len(expected) {
		t.Errorf("Chunk result length = %d, want %d", len(result), len(expected))
	}

	for i, chunk := range result {
		if len(chunk) != len(expected[i]) {
			t.Errorf("Chunk[%d] length = %d, want %d", i, len(chunk), len(expected[i]))
		}
	}
}

func TestFlatten(t *testing.T) {
	slices := [][]string{
		{"a", "b"},
		{"c", "d"},
		{"e"},
	}

	result := Flatten(slices)
	expected := []string{"a", "b", "c", "d", "e"}

	if len(result) != len(expected) {
		t.Errorf("Flatten result length = %d, want %d", len(result), len(expected))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("Flatten[%d] = %s, want %s", i, v, expected[i])
		}
	}
}

func TestReverse(t *testing.T) {
	slice := []string{"a", "b", "c"}
	result := Reverse(slice)

	expected := []string{"c", "b", "a"}
	if len(result) != len(expected) {
		t.Errorf("Reverse result length = %d, want %d", len(result), len(expected))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("Reverse[%d] = %s, want %s", i, v, expected[i])
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Millisecond, "500.00ms"},
		{5 * time.Second, "5.00s"},
		{5 * time.Minute, "5.00m"},
		{5 * time.Hour, "5.00h"},
	}

	for _, tt := range tests {
		result := FormatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
	}

	for _, tt := range tests {
		result := FormatSize(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatSize(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestMinInt(t *testing.T) {
	if MinInt(1, 2) != 1 {
		t.Error("Expected MinInt(1, 2) = 1")
	}
	if MinInt(2, 1) != 1 {
		t.Error("Expected MinInt(2, 1) = 1")
	}
}

func TestMaxInt(t *testing.T) {
	if MaxInt(1, 2) != 2 {
		t.Error("Expected MaxInt(1, 2) = 2")
	}
	if MaxInt(2, 1) != 2 {
		t.Error("Expected MaxInt(2, 1) = 2")
	}
}

func TestClampInt(t *testing.T) {
	if ClampInt(5, 0, 10) != 5 {
		t.Error("Expected ClampInt(5, 0, 10) = 5")
	}
	if ClampInt(-1, 0, 10) != 0 {
		t.Error("Expected ClampInt(-1, 0, 10) = 0")
	}
	if ClampInt(15, 0, 10) != 10 {
		t.Error("Expected ClampInt(15, 0, 10) = 10")
	}
}
