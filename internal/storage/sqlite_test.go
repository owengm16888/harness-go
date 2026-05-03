package storage

import (
	"context"
	"os"
	"testing"

	"github.com/harness-engineering/harness/models"
)

func TestSQLiteStorage_TaskOperations(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "harness-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 创建存储
	store, err := NewSQLiteStorage(DefaultSQLiteConfig(tmpFile.Name()))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 测试保存任务
	taskState := &models.TaskState{
		Task: models.Task{
			ID:          "test-1",
			Type:        "implement",
			Description: "Test task",
		},
		Status: models.TaskStatusPending,
	}

	if err := store.SaveTask(ctx, taskState); err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// 测试获取任务
	retrieved, err := store.GetTask(ctx, "test-1")
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.Task.ID != "test-1" {
		t.Errorf("Expected task ID test-1, got %s", retrieved.Task.ID)
	}

	// 测试列出任务
	tasks, err := store.ListTasks(ctx, models.TaskFilter{})
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// 测试删除任务
	if err := store.DeleteTask(ctx, "test-1"); err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// 验证删除
	_, err = store.GetTask(ctx, "test-1")
	if err == nil {
		t.Error("Expected error after delete, got nil")
	}
}

func TestSQLiteStorage_SessionOperations(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "harness-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 创建存储
	store, err := NewSQLiteStorage(DefaultSQLiteConfig(tmpFile.Name()))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 测试保存会话
	session := &Session{
		ID:          "session-1",
		Environment: "test",
	}

	if err := store.SaveSession(ctx, session); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// 测试获取会话
	retrieved, err := store.GetSession(ctx, "session-1")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved.ID != "session-1" {
		t.Errorf("Expected session ID session-1, got %s", retrieved.ID)
	}

	// 测试列出会话
	sessions, err := store.ListSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	// 测试删除会话
	if err := store.DeleteSession(ctx, "session-1"); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// 验证删除
	_, err = store.GetSession(ctx, "session-1")
	if err == nil {
		t.Error("Expected error after delete, got nil")
	}
}

func TestSQLiteStorage_KnowledgeOperations(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "harness-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 创建存储
	store, err := NewSQLiteStorage(DefaultSQLiteConfig(tmpFile.Name()))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 测试保存知识
	entry := &models.KnowledgeEntry{
		ID:       "knowledge-1",
		Type:     "pattern",
		Title:    "Test Knowledge",
		Content:  "This is test content",
		Tags:     []string{"test", "example"},
		Metadata: map[string]any{"key": "value"},
	}

	if err := store.SaveKnowledge(ctx, entry); err != nil {
		t.Fatalf("Failed to save knowledge: %v", err)
	}

	// 测试获取知识
	retrieved, err := store.GetKnowledge(ctx, "knowledge-1")
	if err != nil {
		t.Fatalf("Failed to get knowledge: %v", err)
	}

	if retrieved.ID != "knowledge-1" {
		t.Errorf("Expected knowledge ID knowledge-1, got %s", retrieved.ID)
	}

	// 测试搜索知识
	results, err := store.SearchKnowledge(ctx, "test", 10)
	if err != nil {
		t.Fatalf("Failed to search knowledge: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results, got 0")
	}

	// 测试删除知识
	if err := store.DeleteKnowledge(ctx, "knowledge-1"); err != nil {
		t.Fatalf("Failed to delete knowledge: %v", err)
	}

	// 验证删除
	_, err = store.GetKnowledge(ctx, "knowledge-1")
	if err == nil {
		t.Error("Expected error after delete, got nil")
	}
}

func TestSQLiteStorage_PatternOperations(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "harness-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 创建存储
	store, err := NewSQLiteStorage(DefaultSQLiteConfig(tmpFile.Name()))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 测试保存模式
	pattern := &models.Pattern{
		ID:          "pattern-1",
		Name:        "Test Pattern",
		Description: "This is a test pattern",
		Trigger:     "test|example",
	}

	if err := store.SavePattern(ctx, pattern); err != nil {
		t.Fatalf("Failed to save pattern: %v", err)
	}

	// 测试获取模式
	retrieved, err := store.GetPattern(ctx, "pattern-1")
	if err != nil {
		t.Fatalf("Failed to get pattern: %v", err)
	}

	if retrieved.ID != "pattern-1" {
		t.Errorf("Expected pattern ID pattern-1, got %s", retrieved.ID)
	}

	// 测试列出模式
	patterns, err := store.ListPatterns(ctx)
	if err != nil {
		t.Fatalf("Failed to list patterns: %v", err)
	}

	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(patterns))
	}

	// 测试删除模式
	if err := store.DeletePattern(ctx, "pattern-1"); err != nil {
		t.Fatalf("Failed to delete pattern: %v", err)
	}

	// 验证删除
	_, err = store.GetPattern(ctx, "pattern-1")
	if err == nil {
		t.Error("Expected error after delete, got nil")
	}
}
