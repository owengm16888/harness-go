package collaboration

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"log/slog"
	"time"

	"github.com/harness-engineering/harness/models"
)

// Registry Agent 注册表 — 管理 Agent 的注册、发现和状态
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*models.Agent
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*models.Agent),
	}
}

// Register 注册 Agent
func (r *Registry) Register(agent *models.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent.ID == "" {
		return fmt.Errorf("registry: agent ID is required")
	}
	if _, exists := r.agents[agent.ID]; exists {
		return fmt.Errorf("registry: agent already registered: %s", agent.ID)
	}

	agent.RegisteredAt = time.Now()
	agent.LastActiveAt = time.Now()
	agent.Status = models.AgentStatusIdle

	r.agents[agent.ID] = agent
	return nil
}

// Unregister 注销 Agent
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[id]; !exists {
		return fmt.Errorf("registry: agent not found: %s", id)
	}

	delete(r.agents, id)
	return nil
}

// Get 获取 Agent
func (r *Registry) Get(id string) (*models.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", id)
	}

	return agent, nil
}

// List 列出所有 Agent
func (r *Registry) List() []*models.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agents []*models.Agent
	for _, a := range r.agents {
		agents = append(agents, a)
	}
	return agents
}

// UpdateStatus 更新 Agent 状态
func (r *Registry) UpdateStatus(id string, status models.AgentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[id]
	if !exists {
		return fmt.Errorf("registry: agent not found: %s", id)
	}

	agent.Status = status
	agent.LastActiveAt = time.Now()
	return nil
}

// AssignTask 标记 Agent 正在执行任务
func (r *Registry) AssignTask(agentID, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	if agent.Status != models.AgentStatusIdle {
		return fmt.Errorf("agent %s is not idle (status: %s)", agentID, agent.Status)
	}

	agent.Status = models.AgentStatusBusy
	agent.CurrentTask = taskID
	agent.LastActiveAt = time.Now()
	return nil
}

// ReleaseTask 标记 Agent 完成任务
func (r *Registry) ReleaseTask(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.Status = models.AgentStatusIdle
	agent.CurrentTask = ""
	agent.LastActiveAt = time.Now()
	return nil
}

// FindByCapability 根据能力查找 Agent（按匹配度排序）
func (r *Registry) FindByCapability(capability string, limit int) []*models.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	type scoredAgent struct {
		agent *models.Agent
		score float64
	}

	var scored []scoredAgent
	for _, agent := range r.agents {
		if agent.Status != models.AgentStatusIdle {
			continue
		}
		for _, cap := range agent.Capabilities {
			if cap.Name == capability {
				scored = append(scored, scoredAgent{agent: agent, score: cap.Confidence})
				break
			}
		}
	}

	// 按置信度降序排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var result []*models.Agent
	for i, s := range scored {
		if limit > 0 && i >= limit {
			break
		}
		result = append(result, s.agent)
	}

	return result
}

// FindByAdapter 根据适配器类型查找 Agent
func (r *Registry) FindByAdapter(adapter string) []*models.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*models.Agent
	for _, agent := range r.agents {
		if agent.Adapter == adapter && agent.Status == models.AgentStatusIdle {
			result = append(result, agent)
		}
	}
	return result
}

// FindByRole 根据角色查找 Agent
func (r *Registry) FindByRole(role models.AgentRole) []*models.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*models.Agent
	for _, agent := range r.agents {
		if agent.Role == role {
			result = append(result, agent)
		}
	}
	return result
}

// GetIdleCount 获取空闲 Agent 数量
func (r *Registry) GetIdleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, agent := range r.agents {
		if agent.Status == models.AgentStatusIdle {
			count++
		}
	}
	return count
}

// AutoSelectAgents 自动选择适合执行任务的 Agent
func (r *Registry) AutoSelectAgents(task models.Task, count int) []*models.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	type scoredAgent struct {
		agent *models.Agent
		score float64
	}

	var scored []scoredAgent
	for _, agent := range r.agents {
		if agent.Status != models.AgentStatusIdle {
			continue
		}

		score := r.calculateFitness(agent, task)
		if score > 0 {
			scored = append(scored, scoredAgent{agent: agent, score: score})
		}
	}

	// 按适配度排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var result []*models.Agent
	for i, s := range scored {
		if count > 0 && i >= count {
			break
		}
		result = append(result, s.agent)
	}

	return result
}

// CheckTimeouts 检查超时的 Agent（应定期调用）
func (r *Registry) CheckTimeouts(timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var timedOut []string
	now := time.Now()

	for id, agent := range r.agents {
		if agent.Status == models.AgentStatusBusy {
			if now.Sub(agent.LastActiveAt) > timeout {
				agent.Status = models.AgentStatusError
				agent.CurrentTask = ""
				timedOut = append(timedOut, id)
				slog.Warn("agent timed out",
					"agent_id", id, "last_active", agent.LastActiveAt,
					"timeout", timeout)
			}
		}
	}

	return timedOut
}

// StartWatcher 启动 Agent 健康监控（后台 goroutine）
func (r *Registry) StartWatcher(ctx context.Context, interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.CheckTimeouts(timeout)
			}
		}
	}()
}

// GetStats 获取注册表统计
func (r *Registry) GetStats() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]int{
		"total":  len(r.agents),
		"idle":   0,
		"busy":   0,
		"offline": 0,
		"error":  0,
	}

	for _, agent := range r.agents {
		switch agent.Status {
		case models.AgentStatusIdle:
			stats["idle"]++
		case models.AgentStatusBusy:
			stats["busy"]++
		case models.AgentStatusOffline:
			stats["offline"]++
		case models.AgentStatusError:
			stats["error"]++
		}
	}

	return stats
}

// calculateFitness 计算 Agent 对任务的适配度
func (r *Registry) calculateFitness(agent *models.Agent, task models.Task) float64 {
	score := 0.0

	// 检查能力匹配
	for _, cap := range agent.Capabilities {
		if cap.Name == task.Type {
			score += cap.Confidence * 2.0 // 能力匹配权重最高
			break
		}
	}

	// 检查领域匹配
	taskDomain, _ := task.Context["domain"].(string)
	if taskDomain != "" {
		for _, cap := range agent.Capabilities {
			for _, domain := range cap.Domains {
				if domain == taskDomain {
					score += 0.5
					break
				}
			}
		}
	}

	// 检查约束兼容性
	for _, taskConstraint := range task.Constraints {
		for _, agentConstraint := range agent.Constraints {
			if taskConstraint.Type == agentConstraint.Type {
				score += 0.2 // 约束匹配加分
			}
		}
	}

	return score
}
