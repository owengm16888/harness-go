package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/harness-engineering/harness/models"
)

// Storage 存储接口
type Storage interface {
	TaskStore
	StateStore
	KnowledgeStore
	PatternStore
	Stats() StorageStats
	Close() error
}

// TaskStore 任务存储接口
type TaskStore interface {
	SaveTask(ctx context.Context, state *models.TaskState) error
	GetTask(ctx context.Context, id string) (*models.TaskState, error)
	ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.TaskState, error)
	DeleteTask(ctx context.Context, id string) error
	BatchSaveTasks(ctx context.Context, states []*models.TaskState) error
}

// StateStore 状态存储接口
type StateStore interface {
	SaveSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context) ([]*Session, error)
	DeleteSession(ctx context.Context, id string) error
}

// KnowledgeStore 知识存储接口
type KnowledgeStore interface {
	SaveKnowledge(ctx context.Context, entry *models.KnowledgeEntry) error
	GetKnowledge(ctx context.Context, id string) (*models.KnowledgeEntry, error)
	ListKnowledge(ctx context.Context, offset, limit int) ([]*models.KnowledgeEntry, error)
	DeleteKnowledge(ctx context.Context, id string) error
	SearchKnowledge(ctx context.Context, query string, limit int) ([]*models.KnowledgeEntry, error)
}

// PatternStore 模式存储接口
type PatternStore interface {
	SavePattern(ctx context.Context, pattern *models.Pattern) error
	GetPattern(ctx context.Context, id string) (*models.Pattern, error)
	ListPatterns(ctx context.Context) ([]*models.Pattern, error)
	DeletePattern(ctx context.Context, id string) error
}

// Session 会话
type Session struct {
	ID          string          `json:"id"`
	Environment string          `json:"environment"`
	State       models.State    `json:"state"`
	History     []StateSnapshot `json:"history"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// StateSnapshot 状态快照
type StateSnapshot struct {
	State     models.State `json:"state"`
	Timestamp string       `json:"timestamp"`
	Reason    string       `json:"reason"`
}

// StorageStats 存储统计
type StorageStats struct {
	TaskCount       int       `json:"task_count"`
	SessionCount    int       `json:"session_count"`
	KnowledgeCount  int       `json:"knowledge_count"`
	PatternCount    int       `json:"pattern_count"`
	DBSize          int64     `json:"db_size"`
	LastUpdated     time.Time `json:"last_updated"`
}

// SQLiteStorageConfig SQLite 配置
type SQLiteStorageConfig struct {
	Path            string `yaml:"path"`
	JournalMode     string `yaml:"journal_mode"`
	BusyTimeout     int    `yaml:"busy_timeout"`
	Synchronous     string `yaml:"synchronous"`
	MmapSize        int    `yaml:"mmap_size"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

// DefaultSQLiteConfig 默认配置
func DefaultSQLiteConfig(path string) SQLiteStorageConfig {
	return SQLiteStorageConfig{
		Path:            path,
		JournalMode:     "WAL",
		BusyTimeout:     5000,
		Synchronous:     "NORMAL",
		MmapSize:        67108864, // 64MB
		MaxOpenConns:    1,        // SQLite 单写者
		MaxIdleConns:    1,
		ConnMaxLifetime: 0,        // 不过期
	}
}

// SQLiteStorage SQLite 存储实现
type SQLiteStorage struct {
	mu     sync.RWMutex
	db     *sql.DB
	config SQLiteStorageConfig
	stats  StorageStats
}

// NewSQLiteStorage 创建 SQLite 存储
func NewSQLiteStorage(config SQLiteStorageConfig) (*SQLiteStorage, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("database path is required")
	}

	// 构建 DSN
	dsn := config.Path
	if config.Path != ":memory:" {
		params := "?"
		if config.JournalMode != "" {
			params += "_journal_mode=" + config.JournalMode + "&"
		}
		if config.BusyTimeout > 0 {
			params += fmt.Sprintf("_busy_timeout=%d&", config.BusyTimeout)
		}
		if config.Synchronous != "" {
			params += "_synchronous=" + config.Synchronous + "&"
		}
		dsn += params
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 连接池设置
	if config.MaxOpenConns <= 0 {
		config.MaxOpenConns = 1
	}
	if config.MaxIdleConns <= 0 {
		config.MaxIdleConns = 1
	}
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)

	// 启用 WAL 模式
	if config.JournalMode == "WAL" {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return nil, fmt.Errorf("failed to enable WAL: %w", err)
		}
	}

	// 启用内存映射
	if config.MmapSize > 0 {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA mmap_size=%d", config.MmapSize)); err != nil {
			slog.Warn("failed to enable mmap", "error", err)
		}
	}

	// 初始化表
	if err := initTables(db); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	slog.Info("SQLite storage initialized", "path", config.Path)

	return &SQLiteStorage{
		db:     db,
		config: config,
	}, nil
}

// initTables 初始化数据库表
func initTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			data TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			environment TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			access_count INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS patterns (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			trigger_text TEXT,
			data TEXT NOT NULL,
			success_rate REAL DEFAULT 0,
			usage_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_updated ON tasks(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_title ON knowledge(title)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_type ON knowledge(type)`,
		`CREATE INDEX IF NOT EXISTS idx_patterns_success ON patterns(success_rate DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_patterns_name ON patterns(name)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// SaveTask 保存任务
func (s *SQLiteStorage) SaveTask(ctx context.Context, state *models.TaskState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	query := `INSERT OR REPLACE INTO tasks (id, data, status, updated_at) VALUES (?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query, state.Task.ID, string(data), string(state.Status), time.Now())
	if err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}

	return nil
}

// BatchSaveTasks 批量保存任务
func (s *SQLiteStorage) BatchSaveTasks(ctx context.Context, states []*models.TaskState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO tasks (id, data, status, updated_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, state := range states {
		data, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("failed to marshal task %s: %w", state.Task.ID, err)
		}

		_, err = stmt.ExecContext(ctx, state.Task.ID, string(data), string(state.Status), time.Now())
		if err != nil {
			return fmt.Errorf("failed to save task %s: %w", state.Task.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTask 获取任务
func (s *SQLiteStorage) GetTask(ctx context.Context, id string) (*models.TaskState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT data FROM tasks WHERE id = ?`
	var data string
	err := s.db.QueryRowContext(ctx, query, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	var state models.TaskState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &state, nil
}

// ListTasks 列出任务
func (s *SQLiteStorage) ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.TaskState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT data FROM tasks WHERE 1=1`
	args := []interface{}{}

	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.Type != "" {
		query += ` AND json_extract(data, '$.task.type') = ?`
		args = append(args, filter.Type)
	}

	query += ` ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*models.TaskState
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		var state models.TaskState
		if err := json.Unmarshal([]byte(data), &state); err != nil {
			continue
		}
		tasks = append(tasks, &state)
	}

	return tasks, nil
}

// DeleteTask 删除任务
func (s *SQLiteStorage) DeleteTask(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM tasks WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	return nil
}

// SaveSession 保存会话
func (s *SQLiteStorage) SaveSession(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	query := `INSERT OR REPLACE INTO sessions (id, environment, data, updated_at) VALUES (?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query, session.ID, session.Environment, string(data), time.Now())
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// GetSession 获取会话
func (s *SQLiteStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT data FROM sessions WHERE id = ?`
	var data string
	err := s.db.QueryRowContext(ctx, query, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// ListSessions 列出会话
func (s *SQLiteStorage) ListSessions(ctx context.Context) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT data FROM sessions ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		var session Session
		if err := json.Unmarshal([]byte(data), &session); err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// DeleteSession 删除会话
func (s *SQLiteStorage) DeleteSession(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM sessions WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", id)
	}

	return nil
}

// SaveKnowledge 保存知识
func (s *SQLiteStorage) SaveKnowledge(ctx context.Context, entry *models.KnowledgeEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tags, _ := json.Marshal(entry.Tags)
	metadata, _ := json.Marshal(entry.Metadata)

	query := `INSERT OR REPLACE INTO knowledge (id, type, title, content, tags, metadata, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, entry.ID, entry.Type, entry.Title, entry.Content, string(tags), string(metadata), time.Now())
	if err != nil {
		return fmt.Errorf("failed to save knowledge: %w", err)
	}

	return nil
}

// GetKnowledge 获取知识
func (s *SQLiteStorage) GetKnowledge(ctx context.Context, id string) (*models.KnowledgeEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, type, title, content, tags, metadata, created_at, updated_at, access_count FROM knowledge WHERE id = ?`
	var entry models.KnowledgeEntry
	var tags, metadata string
	var createdAt, updatedAt string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&entry.ID, &entry.Type, &entry.Title, &entry.Content,
		&tags, &metadata, &createdAt, &updatedAt, &entry.AccessCount,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("knowledge not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge: %w", err)
	}

	if err := json.Unmarshal([]byte(tags), &entry.Tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}
	if err := json.Unmarshal([]byte(metadata), &entry.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &entry, nil
}

// ListKnowledge 列出知识
func (s *SQLiteStorage) ListKnowledge(ctx context.Context, offset, limit int) ([]*models.KnowledgeEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, type, title, content, tags, metadata, created_at, updated_at, access_count FROM knowledge ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list knowledge: %w", err)
	}
	defer rows.Close()

	var entries []*models.KnowledgeEntry
	for rows.Next() {
		var entry models.KnowledgeEntry
		var tags, metadata string
		var createdAt, updatedAt string

		if err := rows.Scan(
			&entry.ID, &entry.Type, &entry.Title, &entry.Content,
			&tags, &metadata, &createdAt, &updatedAt, &entry.AccessCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan knowledge: %w", err)
		}

		if err := json.Unmarshal([]byte(tags), &entry.Tags); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(metadata), &entry.Metadata); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// DeleteKnowledge 删除知识
func (s *SQLiteStorage) DeleteKnowledge(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM knowledge WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete knowledge: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("knowledge not found: %s", id)
	}

	return nil
}

// SearchKnowledge 搜索知识
func (s *SQLiteStorage) SearchKnowledge(ctx context.Context, query string, limit int) ([]*models.KnowledgeEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sqlQuery := `SELECT id, type, title, content, tags, metadata, created_at, updated_at, access_count 
		FROM knowledge 
		WHERE title LIKE ? OR content LIKE ? OR tags LIKE ?
		ORDER BY access_count DESC 
		LIMIT ?`

	searchPattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, sqlQuery, searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge: %w", err)
	}
	defer rows.Close()

	var entries []*models.KnowledgeEntry
	for rows.Next() {
		var entry models.KnowledgeEntry
		var tags, metadata string
		var createdAt, updatedAt string

		if err := rows.Scan(
			&entry.ID, &entry.Type, &entry.Title, &entry.Content,
			&tags, &metadata, &createdAt, &updatedAt, &entry.AccessCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan knowledge: %w", err)
		}

		if err := json.Unmarshal([]byte(tags), &entry.Tags); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(metadata), &entry.Metadata); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// SavePattern 保存模式
func (s *SQLiteStorage) SavePattern(ctx context.Context, pattern *models.Pattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, _ := json.Marshal(pattern)

	query := `INSERT OR REPLACE INTO patterns (id, name, description, trigger_text, data, success_rate, usage_count, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query,
		pattern.ID, pattern.Name, pattern.Description, pattern.Trigger,
		string(data), pattern.SuccessRate, pattern.UsageCount, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save pattern: %w", err)
	}

	return nil
}

// GetPattern 获取模式
func (s *SQLiteStorage) GetPattern(ctx context.Context, id string) (*models.Pattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT data FROM patterns WHERE id = ?`
	var data string
	err := s.db.QueryRowContext(ctx, query, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pattern not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pattern: %w", err)
	}

	var pattern models.Pattern
	if err := json.Unmarshal([]byte(data), &pattern); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pattern: %w", err)
	}

	return &pattern, nil
}

// ListPatterns 列出模式
func (s *SQLiteStorage) ListPatterns(ctx context.Context) ([]*models.Pattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT data FROM patterns ORDER BY success_rate DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list patterns: %w", err)
	}
	defer rows.Close()

	var patterns []*models.Pattern
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan pattern: %w", err)
		}

		var pattern models.Pattern
		if err := json.Unmarshal([]byte(data), &pattern); err != nil {
			continue
		}
		patterns = append(patterns, &pattern)
	}

	return patterns, nil
}

// DeletePattern 删除模式
func (s *SQLiteStorage) DeletePattern(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM patterns WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete pattern: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("pattern not found: %s", id)
	}

	return nil
}

// Stats 获取存储统计
func (s *SQLiteStorage) Stats() StorageStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := StorageStats{
		LastUpdated: time.Now(),
	}

	// 统计任务数量
	s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&stats.TaskCount)
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&stats.SessionCount)
	s.db.QueryRow(`SELECT COUNT(*) FROM knowledge`).Scan(&stats.KnowledgeCount)
	s.db.QueryRow(`SELECT COUNT(*) FROM patterns`).Scan(&stats.PatternCount)

	return stats
}

// Close 关闭存储
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}
