package distributed

import (
	"context"
	"testing"
	"time"
)

func TestCluster_AddNode(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	node := &Node{
		ID:       "node-2",
		Address:  "localhost",
		Port:     8081,
		Status:   NodeStatusActive,
		LastSeen: time.Now(),
	}

	err := cluster.AddNode(node)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	nodes := cluster.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestCluster_RemoveNode(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	node := &Node{
		ID:       "node-2",
		Address:  "localhost",
		Port:     8081,
		Status:   NodeStatusActive,
		LastSeen: time.Now(),
	}

	cluster.AddNode(node)

	err := cluster.RemoveNode("node-2")
	if err != nil {
		t.Fatalf("Failed to remove node: %v", err)
	}

	nodes := cluster.ListNodes()
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}
}

func TestCluster_RemoveSelf(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	err := cluster.RemoveNode("node-1")
	if err == nil {
		t.Error("Expected error when removing self")
	}
}

func TestCluster_GetNode(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	node, err := cluster.GetNode("node-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}

	if node.ID != "node-1" {
		t.Errorf("Expected node ID node-1, got %s", node.ID)
	}
}

func TestCluster_GetSelf(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	self := cluster.GetSelf()
	if self.ID != "node-1" {
		t.Errorf("Expected self ID node-1, got %s", self.ID)
	}
}

func TestCluster_IsLeader(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	// 初始状态下，自身应该是 leader
	if !cluster.IsLeader() {
		t.Error("Expected self to be leader")
	}
}

func TestCluster_GetStats(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	node := &Node{
		ID:       "node-2",
		Address:  "localhost",
		Port:     8081,
		Status:   NodeStatusActive,
		LastSeen: time.Now(),
	}

	cluster.AddNode(node)

	stats := cluster.GetStats()

	if stats.TotalNodes != 2 {
		t.Errorf("Expected 2 total nodes, got %d", stats.TotalNodes)
	}

	if stats.ActiveNodes != 2 {
		t.Errorf("Expected 2 active nodes, got %d", stats.ActiveNodes)
	}

	if stats.Leader != "node-1" {
		t.Errorf("Expected leader node-1, got %s", stats.Leader)
	}
}

func TestCluster_StartStop(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	ctx := context.Background()

	// 启动
	err := cluster.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// 停止
	err = cluster.Stop()
	if err != nil {
		t.Fatalf("Failed to stop cluster: %v", err)
	}
}

func TestTaskDistribution_RoundRobin(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	node := &Node{
		ID:       "node-2",
		Address:  "localhost",
		Port:     8081,
		Status:   NodeStatusActive,
		LastSeen: time.Now(),
	}

	cluster.AddNode(node)

	dist := NewTaskDistribution(cluster, StrategyRoundRobin)

	node1, err := dist.Distribute("task-1")
	if err != nil {
		t.Fatalf("Failed to distribute task: %v", err)
	}

	if node1 == nil {
		t.Error("Expected node, got nil")
	}
}

func TestTaskDistribution_NoActiveNodes(t *testing.T) {
	cfg := ClusterConfig{
		NodeID:  "node-1",
		Address: "localhost",
		Port:    8080,
	}

	cluster := NewCluster(cfg)

	// 将自身设置为非活跃
	self, _ := cluster.GetNode("node-1")
	self.Status = NodeStatusInactive

	dist := NewTaskDistribution(cluster, StrategyRoundRobin)

	_, err := dist.Distribute("task-1")
	if err == nil {
		t.Error("Expected error when no active nodes")
	}
}
