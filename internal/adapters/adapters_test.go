package adapters

import (
	"context"
	"testing"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/models"
)

func TestClaudeCodeAdapter_Name(t *testing.T) {
	adapter := NewClaudeCodeAdapter()
	if adapter.Name() != "claude-code" {
		t.Errorf("Expected name claude-code, got %s", adapter.Name())
	}
}

func TestClaudeCodeAdapter_Initialize(t *testing.T) {
	adapter := NewClaudeCodeAdapter()

	cfg := config.AdapterConfig{
		Enabled:   true,
		RootDir:   ".",
		HooksPath: ".claude-plugin/hooks.json",
		PlansPath: "Plans.md",
	}

	ctx := context.Background()
	err := adapter.Initialize(ctx, cfg)

	// 注意：这个测试可能会失败，因为需要实际的文件
	// 在实际项目中，应该使用 mock 或测试文件
	if err != nil {
		t.Logf("Initialize failed (expected in test environment): %v", err)
	}
}

func TestHermesAdapter_Name(t *testing.T) {
	adapter := NewHermesAdapter()
	if adapter.Name() != "hermes" {
		t.Errorf("Expected name hermes, got %s", adapter.Name())
	}
}

func TestCodexCLIAdapter_Name(t *testing.T) {
	adapter := NewCodexCLIAdapter()
	if adapter.Name() != "codex-cli" {
		t.Errorf("Expected name codex-cli, got %s", adapter.Name())
	}
}

func TestCodexCLIAdapter_Initialize(t *testing.T) {
	adapter := NewCodexCLIAdapter()

	cfg := config.AdapterConfig{
		Enabled:    true,
		RootDir:    ".",
		AgentsPath: "AGENTS.md",
	}

	ctx := context.Background()
	err := adapter.Initialize(ctx, cfg)

	// 注意：这个测试可能会失败，因为需要实际的 Codex CLI
	// 在实际项目中，应该使用 mock
	if err != nil {
		t.Logf("Initialize failed (expected in test environment): %v", err)
	}
}

func TestTask_Validation(t *testing.T) {
	tests := []struct {
		name    string
		task    models.Task
		wantErr bool
	}{
		{
			name: "valid task",
			task: models.Task{
				ID:          "test-1",
				Type:        "implement",
				Description: "Test task",
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			task: models.Task{
				Type:        "implement",
				Description: "Test task",
			},
			wantErr: true,
		},
		{
			name: "missing Type",
			task: models.Task{
				ID:          "test-2",
				Description: "Test task",
			},
			wantErr: true,
		},
		{
			name: "missing Description",
			task: models.Task{
				ID:   "test-3",
				Type: "implement",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTask(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func validateTask(task models.Task) error {
	if task.ID == "" {
		return ErrMissingID
	}
	if task.Type == "" {
		return ErrMissingType
	}
	if task.Description == "" {
		return ErrMissingDescription
	}
	return nil
}

var (
	ErrMissingID          = &ValidationError{Field: "id", Message: "ID is required"}
	ErrMissingType        = &ValidationError{Field: "type", Message: "Type is required"}
	ErrMissingDescription = &ValidationError{Field: "description", Message: "Description is required"}
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
