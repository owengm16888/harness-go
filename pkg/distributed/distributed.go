package distributed

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Node 节点
type Node struct {
	ID        string            `json:"id"`
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Status    NodeStatus        `json:"status"`
	LastSeen  time.Time         `json:"last_seen"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusActive   NodeStatus = "active"
	NodeStatusInactive NodeStatus = "inactive"
	NodeStatusLeaving  NodeStatus = "leaving"
	NodeStatusFailed   NodeStatus = "failed"
)

// Cluster 集群
type Cluster struct {
	mu       sync.RWMutex
	nodes    map[string]*Node
	self     *Node
	leader   string
	config   ClusterConfig
	stopChan chan struct{}
}

// ClusterConfig 集群配置
type ClusterConfig struct {
	NodeID           string        `yaml:"node_id"`
	Address          string        `yaml:"address"`
	Port             int           `yaml:"port"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout"`
	MaxNodes         int           `yaml:"max_nodes"`
}

// NewCluster 创建集群
func NewCluster(cfg ClusterConfig) *Cluster {
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 5 * time.Second
	}
	if cfg.HeartbeatTimeout == 0 {
		cfg.HeartbeatTimeout = 15 * time.Second
	}
	if cfg.MaxNodes == 0 {
		cfg.MaxNodes = 100
	}

	self := &Node{
		ID:       cfg.NodeID,
		Address:  cfg.Address,
		Port:     cfg.Port,
		Status:   NodeStatusActive,
		LastSeen: time.Now(),
		Metadata: make(map[string]string),
	}

	return &Cluster{
		nodes:    map[string]*Node{cfg.NodeID: self},
		leader:   cfg.NodeID,
		self:     self,
		config:   cfg,
		stopChan: make(chan struct{}),
	}
}

// Start 启动集群
func (c *Cluster) Start(ctx context.Context) error {
	// 启动心跳
	go c.heartbeat(ctx)

	// 启动节点监控
	go c.monitor(ctx)

	return nil
}

// Stop 停止集群
func (c *Cluster) Stop() error {
	close(c.stopChan)
	return nil
}

// AddNode 添加节点
func (c *Cluster) AddNode(node *Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.nodes) >= c.config.MaxNodes {
		return fmt.Errorf("cluster is full")
	}

	if _, exists := c.nodes[node.ID]; exists {
		return fmt.Errorf("node already exists: %s", node.ID)
	}

	c.nodes[node.ID] = node
	return nil
}

// RemoveNode 移除节点
func (c *Cluster) RemoveNode(nodeID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if nodeID == c.self.ID {
		return fmt.Errorf("cannot remove self")
	}

	if _, exists := c.nodes[nodeID]; !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	delete(c.nodes, nodeID)

	// 如果移除的是 leader，选举新 leader
	if c.leader == nodeID {
		c.electLeader()
	}

	return nil
}

// GetNode 获取节点
func (c *Cluster) GetNode(nodeID string) (*Node, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	node, exists := c.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return node, nil
}

// ListNodes 列出节点
func (c *Cluster) ListNodes() []*Node {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodes := make([]*Node, 0, len(c.nodes))
	for _, node := range c.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// GetLeader 获取 leader
func (c *Cluster) GetLeader() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.leader
}

// IsLeader 检查是否是 leader
func (c *Cluster) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.leader == c.self.ID
}

// GetSelf 获取自身节点
func (c *Cluster) GetSelf() *Node {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.self
}

// GetStats 获取统计信息
func (c *Cluster) GetStats() *ClusterStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &ClusterStats{
		TotalNodes: len(c.nodes),
		Leader:     c.leader,
		SelfID:     c.self.ID,
	}

	for _, node := range c.nodes {
		switch node.Status {
		case NodeStatusActive:
			stats.ActiveNodes++
		case NodeStatusInactive:
			stats.InactiveNodes++
		case NodeStatusFailed:
			stats.FailedNodes++
		}
	}

	return stats
}

// heartbeat 心跳
func (c *Cluster) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (c *Cluster) sendHeartbeat() {
	c.mu.Lock()
	c.self.LastSeen = time.Now()
	c.mu.Unlock()

	// 在实际实现中，这里会向其他节点发送心跳
	// 这里只是更新自身状态
}

// monitor 监控节点
func (c *Cluster) monitor(ctx context.Context) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.checkNodes()
		}
	}
}

// checkNodes 检查节点状态
func (c *Cluster) checkNodes() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, node := range c.nodes {
		if node.ID == c.self.ID {
			continue
		}

		// 检查是否超时
		if now.Sub(node.LastSeen) > c.config.HeartbeatTimeout {
			if node.Status == NodeStatusActive {
				node.Status = NodeStatusInactive
			}
		}

		// 检查是否失败
		if now.Sub(node.LastSeen) > c.config.HeartbeatTimeout*2 {
			node.Status = NodeStatusFailed
		}
	}

	// 如果 leader 失败，选举新 leader
	if leader, exists := c.nodes[c.leader]; exists {
		if leader.Status == NodeStatusFailed {
			c.electLeader()
		}
	}
}

// electLeader 选举 leader
func (c *Cluster) electLeader() {
	// 简单选举：选择第一个活跃节点
	for _, node := range c.nodes {
		if node.Status == NodeStatusActive {
			c.leader = node.ID
			return
		}
	}

	// 如果没有活跃节点，设置自身为 leader
	c.leader = c.self.ID
}

// ClusterStats 集群统计
type ClusterStats struct {
	TotalNodes   int    `json:"total_nodes"`
	ActiveNodes  int    `json:"active_nodes"`
	InactiveNodes int   `json:"inactive_nodes"`
	FailedNodes  int    `json:"failed_nodes"`
	Leader       string `json:"leader"`
	SelfID       string `json:"self_id"`
}

// TaskDistribution 任务分配
type TaskDistribution struct {
	mu      sync.RWMutex
	cluster *Cluster
	strategy DistributionStrategy
}

// DistributionStrategy 分配策略
type DistributionStrategy string

const (
	StrategyRoundRobin DistributionStrategy = "round_robin"
	StrategyRandom     DistributionStrategy = "random"
	StrategyLeastLoad  DistributionStrategy = "least_load"
	StrategyHash       DistributionStrategy = "hash"
)

// NewTaskDistribution 创建任务分配
func NewTaskDistribution(cluster *Cluster, strategy DistributionStrategy) *TaskDistribution {
	return &TaskDistribution{
		cluster:  cluster,
		strategy: strategy,
	}
}

// Distribute 分配任务
func (td *TaskDistribution) Distribute(taskID string) (*Node, error) {
	td.mu.RLock()
	defer td.mu.RUnlock()

	nodes := td.cluster.ListNodes()
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	// 过滤活跃节点
	activeNodes := []*Node{}
	for _, node := range nodes {
		if node.Status == NodeStatusActive {
			activeNodes = append(activeNodes, node)
		}
	}

	if len(activeNodes) == 0 {
		return nil, fmt.Errorf("no active nodes")
	}

	switch td.strategy {
	case StrategyRoundRobin:
		return td.roundRobin(activeNodes), nil
	case StrategyRandom:
		return td.random(activeNodes), nil
	case StrategyLeastLoad:
		return td.leastLoad(activeNodes), nil
	case StrategyHash:
		return td.hash(activeNodes, taskID), nil
	default:
		return td.roundRobin(activeNodes), nil
	}
}

// roundRobin 轮询
func (td *TaskDistribution) roundRobin(nodes []*Node) *Node {
	// 简单实现：使用时间戳
	index := time.Now().UnixNano() % int64(len(nodes))
	return nodes[index]
}

// random 随机
func (td *TaskDistribution) random(nodes []*Node) *Node {
	index := time.Now().UnixNano() % int64(len(nodes))
	return nodes[index]
}

// leastLoad 最少负载
func (td *TaskDistribution) leastLoad(nodes []*Node) *Node {
	// 简单实现：返回第一个节点
	// 实际实现应该跟踪每个节点的负载
	return nodes[0]
}

// hash 哈希
func (td *TaskDistribution) hash(nodes []*Node, taskID string) *Node {
	// 简单哈希
	hash := 0
	for _, c := range taskID {
		hash = hash*31 + int(c)
	}
	index := hash % len(nodes)
	return nodes[index]
}
