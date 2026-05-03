package routine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ============================================================
// Routine 持久化存储
// ============================================================

// RoutineStore Routine 存储接口
type RoutineStore interface {
	// Save 保存实例
	Save(ctx context.Context, instance *RoutineInstance) error

	// Get 获取实例
	Get(ctx context.Context, id string) (*RoutineInstance, error)

	// List 列出实例
	List(ctx context.Context, filter StoreFilter) ([]*RoutineInstance, error)

	// Delete 删除实例
	Delete(ctx context.Context, id string) error

	// SaveReport 保存报告
	SaveReport(ctx context.Context, id string, report *FinalReport) error

	// GetReport 获取报告
	GetReport(ctx context.Context, id string) (*FinalReport, error)
}

// StoreFilter 存储过滤器
type StoreFilter struct {
	Status RoutineStatus `json:"status,omitempty"`
	Type   RoutineType   `json:"type,omitempty"`
	Limit  int           `json:"limit,omitempty"`
	Offset int           `json:"offset,omitempty"`
}

// ============================================================
// 内存存储实现
// ============================================================

// MemoryRoutineStore 内存 Routine 存储
type MemoryRoutineStore struct {
	mu        sync.RWMutex
	instances map[string]*RoutineInstance
	reports   map[string]*FinalReport
}

// NewMemoryRoutineStore 创建内存存储
func NewMemoryRoutineStore() *MemoryRoutineStore {
	return &MemoryRoutineStore{
		instances: make(map[string]*RoutineInstance),
		reports:   make(map[string]*FinalReport),
	}
}

func (s *MemoryRoutineStore) Save(ctx context.Context, instance *RoutineInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.instances[instance.ID] = instance
	slog.Debug("routine saved", "id", instance.ID)
	return nil
}

func (s *MemoryRoutineStore) Get(ctx context.Context, id string) (*RoutineInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, exists := s.instances[id]
	if !exists {
		return nil, fmt.Errorf("routine not found: %s", id)
	}

	return instance, nil
}

func (s *MemoryRoutineStore) List(ctx context.Context, filter StoreFilter) ([]*RoutineInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*RoutineInstance
	for _, instance := range s.instances {
		if filter.Status != "" && instance.Status != filter.Status {
			continue
		}
		if filter.Type != "" && instance.Config.Type != filter.Type {
			continue
		}
		result = append(result, instance)
	}

	// 分页
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (s *MemoryRoutineStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.instances[id]; !exists {
		return fmt.Errorf("routine not found: %s", id)
	}

	delete(s.instances, id)
	delete(s.reports, id)
	return nil
}

func (s *MemoryRoutineStore) SaveReport(ctx context.Context, id string, report *FinalReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reports[id] = report
	return nil
}

func (s *MemoryRoutineStore) GetReport(ctx context.Context, id string) (*FinalReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report, exists := s.reports[id]
	if !exists {
		return nil, fmt.Errorf("report not found: %s", id)
	}

	return report, nil
}

// ============================================================
// 文件存储实现
// ============================================================

// FileRoutineStore 文件 Routine 存储
type FileRoutineStore struct {
	mu        sync.RWMutex
	dir       string
	instances map[string]*RoutineInstance
	reports   map[string]*FinalReport
}

// NewFileRoutineStore 创建文件存储
func NewFileRoutineStore(dir string) *FileRoutineStore {
	store := &FileRoutineStore{
		dir:       dir,
		instances: make(map[string]*RoutineInstance),
		reports:   make(map[string]*FinalReport),
	}

	// 加载已有数据
	store.loadFromDisk()

	return store
}

func (s *FileRoutineStore) loadFromDisk() {
	// TODO: 从文件加载
	slog.Debug("loading routines from disk", "dir", s.dir)
}

func (s *FileRoutineStore) Save(ctx context.Context, instance *RoutineInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.instances[instance.ID] = instance

	// 保存到文件
	if err := s.saveToFile(instance.ID, instance); err != nil {
		return fmt.Errorf("failed to save to file: %w", err)
	}

	return nil
}

func (s *FileRoutineStore) saveToFile(id string, data any) error {
	// TODO: 保存到文件
	slog.Debug("saving to file", "id", id)
	return nil
}

func (s *FileRoutineStore) Get(ctx context.Context, id string) (*RoutineInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, exists := s.instances[id]
	if !exists {
		return nil, fmt.Errorf("routine not found: %s", id)
	}

	return instance, nil
}

func (s *FileRoutineStore) List(ctx context.Context, filter StoreFilter) ([]*RoutineInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*RoutineInstance
	for _, instance := range s.instances {
		if filter.Status != "" && instance.Status != filter.Status {
			continue
		}
		if filter.Type != "" && instance.Config.Type != filter.Type {
			continue
		}
		result = append(result, instance)
	}

	return result, nil
}

func (s *FileRoutineStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.instances[id]; !exists {
		return fmt.Errorf("routine not found: %s", id)
	}

	delete(s.instances, id)
	delete(s.reports, id)

	// TODO: 删除文件

	return nil
}

func (s *FileRoutineStore) SaveReport(ctx context.Context, id string, report *FinalReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reports[id] = report
	return nil
}

func (s *FileRoutineStore) GetReport(ctx context.Context, id string) (*FinalReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report, exists := s.reports[id]
	if !exists {
		return nil, fmt.Errorf("report not found: %s", id)
	}

	return report, nil
}

// ============================================================
// Routine 管理器 — 统一管理
// ============================================================

// RoutineManager Routine 管理器
type RoutineManager struct {
	engine  *DefaultRoutineEngine
	store   RoutineStore
	loader  *ConfigLoader
	report  *ReportGenerator
}

// RoutineManagerConfig 管理器配置
type RoutineManagerConfig struct {
	EngineConfig   EngineConfig
	StoreType      string // memory, file
	StoreDir       string
	ConfigDir      string
	ReportFormat   string
}

// NewRoutineManager 创建 Routine 管理器
func NewRoutineManager(config RoutineManagerConfig) *RoutineManager {
	// 创建引擎
	engine := NewRoutineEngine(config.EngineConfig)

	// 创建存储
	var store RoutineStore
	switch config.StoreType {
	case "file":
		store = NewFileRoutineStore(config.StoreDir)
	default:
		store = NewMemoryRoutineStore()
	}

	// 创建加载器
	loader := NewConfigLoader(config.ConfigDir)

	// 创建报告生成器
	report := NewReportGenerator(config.ReportFormat)

	return &RoutineManager{
		engine: engine,
		store:  store,
		loader: loader,
		report: report,
	}
}

// CreateFromConfig 从配置创建
func (m *RoutineManager) CreateFromConfig(ctx context.Context, configName string, params map[string]any) (*RoutineInstance, error) {
	// 加载配置
	config, err := m.loader.Load(configName)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 应用参数
	if name, ok := params["name"].(string); ok {
		config.Name = name
	}
	if focus, ok := params["focus"].(string); ok {
		if config.Input == nil {
			config.Input = make(map[string]any)
		}
		config.Input["focus"] = focus
	}

	// 创建实例
	instance, err := m.engine.Create(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("failed to create routine: %w", err)
	}

	// 保存到存储
	if err := m.store.Save(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to save routine: %w", err)
	}

	return instance, nil
}

// CreateFromTemplate 从模板创建
func (m *RoutineManager) CreateFromTemplate(ctx context.Context, templateName string, params map[string]any) (*RoutineInstance, error) {
	// 创建配置
	config, err := m.loader.CreateFromTemplate(templateName, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create from template: %w", err)
	}

	// 创建实例
	instance, err := m.engine.Create(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("failed to create routine: %w", err)
	}

	// 保存到存储
	if err := m.store.Save(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to save routine: %w", err)
	}

	return instance, nil
}

// Start 启动
func (m *RoutineManager) Start(ctx context.Context, id string) error {
	return m.engine.Start(ctx, id)
}

// SubmitAnswer 提交回答
func (m *RoutineManager) SubmitAnswer(ctx context.Context, id string, answer string) error {
	return m.engine.SubmitAnswer(ctx, id, answer)
}

// GetQuestion 获取问题
func (m *RoutineManager) GetQuestion(ctx context.Context, id string) (string, error) {
	return m.engine.GetNextQuestion(ctx, id)
}

// GetInstance 获取实例
func (m *RoutineManager) GetInstance(ctx context.Context, id string) (*RoutineInstance, error) {
	return m.store.Get(ctx, id)
}

// ListInstances 列出实例
func (m *RoutineManager) ListInstances(ctx context.Context, filter StoreFilter) ([]*RoutineInstance, error) {
	return m.store.List(ctx, filter)
}

// GetReport 获取报告
func (m *RoutineManager) GetReport(ctx context.Context, id string) (string, error) {
	instance, err := m.store.Get(ctx, id)
	if err != nil {
		return "", err
	}

	return m.report.Generate(instance), nil
}

// GetMarkdownReport 获取 Markdown 报告
func (m *RoutineManager) GetMarkdownReport(ctx context.Context, id string) (string, error) {
	instance, err := m.store.Get(ctx, id)
	if err != nil {
		return "", err
	}

	gen := NewReportGenerator("markdown")
	return gen.Generate(instance), nil
}

// GetHTMLReport 获取 HTML 报告
func (m *RoutineManager) GetHTMLReport(ctx context.Context, id string) (string, error) {
	instance, err := m.store.Get(ctx, id)
	if err != nil {
		return "", err
	}

	gen := NewReportGenerator("html")
	return gen.Generate(instance), nil
}

// Stop 停止
func (m *RoutineManager) Stop(ctx context.Context, id string) error {
	return m.engine.Stop(ctx, id)
}

// Delete 删除
func (m *RoutineManager) Delete(ctx context.Context, id string) error {
	return m.store.Delete(ctx, id)
}

// GetStats 获取统计
func (m *RoutineManager) GetStats(ctx context.Context) map[string]any {
	instances, _ := m.store.List(ctx, StoreFilter{})

	stats := map[string]any{
		"total":  len(instances),
		"by_status": map[string]int{
			"pending":   0,
			"running":   0,
			"completed": 0,
			"failed":    0,
		},
		"by_type": map[string]int{
			"interview":    0,
			"code_review":  0,
			"debugging":    0,
			"architecture": 0,
		},
	}

	byStatus := stats["by_status"].(map[string]int)
	byType := stats["by_type"].(map[string]int)

	for _, instance := range instances {
		byStatus[string(instance.Status)]++
		byType[string(instance.Config.Type)]++
	}

	return stats
}

// Helper function to marshal/unmarshal for storage
func marshalInstance(instance *RoutineInstance) ([]byte, error) {
	return json.Marshal(instance)
}

func unmarshalInstance(data []byte) (*RoutineInstance, error) {
	var instance RoutineInstance
	if err := json.Unmarshal(data, &instance); err != nil {
		return nil, err
	}
	return &instance, nil
}

// GetStartTime 获取开始时间 (helper)
func (ri *RoutineInstance) GetStartTime() time.Time {
	return ri.StartTime
}

// GetEndTime 获取结束时间 (helper)
func (ri *RoutineInstance) GetEndTime() *time.Time {
	return ri.EndTime
}

// GetDuration 获取持续时间 (helper)
func (ri *RoutineInstance) GetDuration() time.Duration {
	if ri.EndTime != nil {
		return ri.EndTime.Sub(ri.StartTime)
	}
	return time.Since(ri.StartTime)
}
