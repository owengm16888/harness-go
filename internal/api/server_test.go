package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harness-engineering/harness/config"
	"github.com/harness-engineering/harness/internal/core"
	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/models"
)

type mockStorage struct {
	tasks    map[string]*models.TaskState
	sessions map[string]*storage.Session
}

func (m *mockStorage) SaveTask(ctx context.Context, state *models.TaskState) error {
	if m.tasks == nil {
		m.tasks = make(map[string]*models.TaskState)
	}
	m.tasks[state.Task.ID] = state
	return nil
}

func (m *mockStorage) GetTask(ctx context.Context, id string) (*models.TaskState, error) {
	if state, ok := m.tasks[id]; ok {
		return state, nil
	}
	return nil, fmt.Errorf("task not found: %s", id)
}

func (m *mockStorage) ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.TaskState, error) {
	var tasks []*models.TaskState
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (m *mockStorage) DeleteTask(ctx context.Context, id string) error {
	delete(m.tasks, id)
	return nil
}

func (m *mockStorage) BatchSaveTasks(ctx context.Context, states []*models.TaskState) error {
	for _, s := range states {
		m.SaveTask(ctx, s)
	}
	return nil
}

func (m *mockStorage) SaveSession(ctx context.Context, session *storage.Session) error { return nil }
func (m *mockStorage) GetSession(ctx context.Context, id string) (*storage.Session, error) { return nil, nil }
func (m *mockStorage) ListSessions(ctx context.Context) ([]*storage.Session, error) { return nil, nil }
func (m *mockStorage) DeleteSession(ctx context.Context, id string) error { return nil }
func (m *mockStorage) SaveKnowledge(ctx context.Context, entry *models.KnowledgeEntry) error { return nil }
func (m *mockStorage) GetKnowledge(ctx context.Context, id string) (*models.KnowledgeEntry, error) { return nil, nil }
func (m *mockStorage) ListKnowledge(ctx context.Context, offset, limit int) ([]*models.KnowledgeEntry, error) { return nil, nil }
func (m *mockStorage) DeleteKnowledge(ctx context.Context, id string) error { return nil }
func (m *mockStorage) SearchKnowledge(ctx context.Context, query string, limit int) ([]*models.KnowledgeEntry, error) { return nil, nil }
func (m *mockStorage) SavePattern(ctx context.Context, pattern *models.Pattern) error { return nil }
func (m *mockStorage) GetPattern(ctx context.Context, id string) (*models.Pattern, error) { return nil, nil }
func (m *mockStorage) ListPatterns(ctx context.Context) ([]*models.Pattern, error) { return nil, nil }
func (m *mockStorage) DeletePattern(ctx context.Context, id string) error { return nil }
func (m *mockStorage) Stats() storage.StorageStats { return storage.StorageStats{} }
func (m *mockStorage) Close() error { return nil }

func setupTestServerLocal(t *testing.T) (*Server, *core.Engine) {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{Addr: ":8080"},
		Engine: config.EngineConfig{MaxConcurrentTasks: 10, RetryCount: 3},
		Storage: config.StorageConfig{Type: "sqlite", Path: ":memory:"},
		Feedback: config.FeedbackConfig{MaxRetries: 3},
		Patterns: config.PatternsConfig{MinSamples: 5, Threshold: 0.7},
		Monitor: config.MonitorConfig{MetricsPort: 9090, LogLevel: "info"},
	}
	store := &mockStorage{}

	engine, err := core.NewEngine(cfg, store)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	server := NewServer(cfg, engine)
	return server, engine
}

func TestHealthCheck(t *testing.T) {
	server, _ := setupTestServerLocal(t)

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
