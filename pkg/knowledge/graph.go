package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// EntityType 知识实体类型
type EntityType string

const (
	EntityConcept   EntityType = "concept"   // 概念
	EntityPattern   EntityType = "pattern"   // 模式
	EntitySolution  EntityType = "solution"  // 解决方案
	EntityError     EntityType = "error"     // 错误
	EntityTool      EntityType = "tool"      // 工具
	EntityPractice  EntityType = "practice"  // 实践
)

// RelationType 关系类型
type RelationType string

const (
	RelationDependsOn    RelationType = "depends_on"    // 依赖
	RelationSimilarTo    RelationType = "similar_to"    // 相似
	RelationSolves       RelationType = "solves"        // 解决
	RelationContradicts  RelationType = "contradicts"    // 矛盾
	RelationImplements   RelationType = "implements"     // 实现
	RelationPartOf       RelationType = "part_of"        // 属于
	RelationCausedBy     RelationType = "caused_by"      // 由...引起
	RelationLeadsTo      RelationType = "leads_to"       // 导致
)

// Entity 知识实体
type Entity struct {
	ID         string            `json:"id"`
	Type       EntityType        `json:"type"`
	Name       string            `json:"name"`
	Content    string            `json:"content"`
	Tags       []string          `json:"tags"`
	Properties map[string]string `json:"properties"`
	Weight     float64           `json:"weight"`     // 权重/重要度
	AccessCount int              `json:"access_count"` // 访问次数
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Relation 知识关系
type Relation struct {
	ID         string       `json:"id"`
	From       string       `json:"from"`       // 源实体 ID
	To         string       `json:"to"`         // 目标实体 ID
	Type       RelationType `json:"type"`
	Weight     float64      `json:"weight"`     // 关系强度
	Properties map[string]string `json:"properties"`
	CreatedAt  time.Time    `json:"created_at"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Entity   *Entity   `json:"entity"`
	Score    float64   `json:"score"`
	Path     []string  `json:"path,omitempty"` // 关系路径
}

// GraphStats 图谱统计
type GraphStats struct {
	EntityCount    int                    `json:"entity_count"`
	RelationCount  int                    `json:"relation_count"`
	TypeCounts     map[EntityType]int     `json:"type_counts"`
	RelationCounts map[RelationType]int   `json:"relation_counts"`
	TopEntities    []*Entity              `json:"top_entities"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// KnowledgeGraph 知识图谱
type KnowledgeGraph struct {
	mu        sync.RWMutex
	entities  map[string]*Entity
	relations map[string]*Relation
	// 索引: 按类型
	typeIndex map[EntityType][]string
	// 索引: 按标签
	tagIndex map[string][]string
	// 索引: 按名称(用于搜索)
	nameIndex map[string][]string
	// 邻接表: entity -> relations
	outEdges map[string][]string
	inEdges  map[string][]string
}

// NewKnowledgeGraph 创建知识图谱
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		entities:      make(map[string]*Entity),
		relations:     make(map[string]*Relation),
		typeIndex:     make(map[EntityType][]string),
		tagIndex:      make(map[string][]string),
		nameIndex:     make(map[string][]string),
		outEdges:      make(map[string][]string),
		inEdges:       make(map[string][]string),
	}
}

// AddEntity 添加实体
func (kg *KnowledgeGraph) AddEntity(ctx context.Context, entity *Entity) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if entity.ID == "" {
		return fmt.Errorf("entity ID is required")
	}

	if _, exists := kg.entities[entity.ID]; exists {
		return fmt.Errorf("entity already exists: %s", entity.ID)
	}

	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now
	if entity.Properties == nil {
		entity.Properties = make(map[string]string)
	}

	kg.entities[entity.ID] = entity

	// 更新索引
	kg.typeIndex[entity.Type] = append(kg.typeIndex[entity.Type], entity.ID)
	for _, tag := range entity.Tags {
		kg.tagIndex[strings.ToLower(tag)] = append(kg.tagIndex[strings.ToLower(tag)], entity.ID)
	}
	// 名称索引 (分词)
	for _, word := range tokenize(entity.Name) {
		kg.nameIndex[strings.ToLower(word)] = append(kg.nameIndex[strings.ToLower(word)], entity.ID)
	}

	return nil
}

// GetEntity 获取实体
func (kg *KnowledgeGraph) GetEntity(ctx context.Context, id string) (*Entity, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	entity, exists := kg.entities[id]
	if !exists {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	// 增加访问计数
	entity.AccessCount++

	return entity, nil
}

// UpdateEntity 更新实体
func (kg *KnowledgeGraph) UpdateEntity(ctx context.Context, id string, update *Entity) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	entity, exists := kg.entities[id]
	if !exists {
		return fmt.Errorf("entity not found: %s", id)
	}

	if update.Name != "" {
		entity.Name = update.Name
	}
	if update.Content != "" {
		entity.Content = update.Content
	}
	if update.Tags != nil {
		entity.Tags = update.Tags
	}
	if update.Weight > 0 {
		entity.Weight = update.Weight
	}
	entity.UpdatedAt = time.Now()

	return nil
}

// DeleteEntity 删除实体
func (kg *KnowledgeGraph) DeleteEntity(ctx context.Context, id string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	entity, exists := kg.entities[id]
	if !exists {
		return fmt.Errorf("entity not found: %s", id)
	}

	// 删除相关关系
	outEdges := kg.outEdges[id]
	inEdges := kg.inEdges[id]
	for _, relID := range append(outEdges, inEdges...) {
		delete(kg.relations, relID)
	}
	delete(kg.outEdges, id)
	delete(kg.inEdges, id)

	// 从索引中删除
	kg.removeFromTypeIndex(entity.Type, id)
	for _, tag := range entity.Tags {
		kg.removeFromTagIndex(strings.ToLower(tag), id)
	}
	for _, word := range tokenize(entity.Name) {
		kg.removeFromNameIndex(strings.ToLower(word), id)
	}

	delete(kg.entities, id)
	return nil
}

// AddRelation 添加关系
func (kg *KnowledgeGraph) AddRelation(ctx context.Context, relation *Relation) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if relation.ID == "" {
		return fmt.Errorf("relation ID is required")
	}

	// 验证实体存在
	if _, exists := kg.entities[relation.From]; !exists {
		return fmt.Errorf("source entity not found: %s", relation.From)
	}
	if _, exists := kg.entities[relation.To]; !exists {
		return fmt.Errorf("target entity not found: %s", relation.To)
	}

	relation.CreatedAt = time.Now()
	if relation.Properties == nil {
		relation.Properties = make(map[string]string)
	}

	kg.relations[relation.ID] = relation
	kg.outEdges[relation.From] = append(kg.outEdges[relation.From], relation.ID)
	kg.inEdges[relation.To] = append(kg.inEdges[relation.To], relation.ID)

	return nil
}

// GetRelations 获取实体的关系
func (kg *KnowledgeGraph) GetRelations(ctx context.Context, entityID string) ([]*Relation, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	if _, exists := kg.entities[entityID]; !exists {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	var relations []*Relation
	for _, relID := range kg.outEdges[entityID] {
		if rel, exists := kg.relations[relID]; exists {
			relations = append(relations, rel)
		}
	}
	for _, relID := range kg.inEdges[entityID] {
		if rel, exists := kg.relations[relID]; exists {
			relations = append(relations, rel)
		}
	}

	return relations, nil
}

// GetNeighbors 获取邻居实体
func (kg *KnowledgeGraph) GetNeighbors(ctx context.Context, entityID string, depth int) ([]*Entity, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	if _, exists := kg.entities[entityID]; !exists {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	visited := make(map[string]bool)
	var result []*Entity

	var bfs func(id string, currentDepth int)
	bfs = func(id string, currentDepth int) {
		if currentDepth > depth || visited[id] {
			return
		}
		visited[id] = true

		if id != entityID {
			if entity, exists := kg.entities[id]; exists {
				result = append(result, entity)
			}
		}

		if currentDepth < depth {
			// 出边
			for _, relID := range kg.outEdges[id] {
				if rel, exists := kg.relations[relID]; exists {
					bfs(rel.To, currentDepth+1)
				}
			}
			// 入边
			for _, relID := range kg.inEdges[id] {
				if rel, exists := kg.relations[relID]; exists {
					bfs(rel.From, currentDepth+1)
				}
			}
		}
	}

	bfs(entityID, 0)
	return result, nil
}

// Search 搜索知识
func (kg *KnowledgeGraph) Search(ctx context.Context, query string, limit int) ([]*SearchResult, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	// 评分: 每个实体的匹配分数
	scores := make(map[string]float64)

	// 1. 名称匹配 (权重最高)
	words := tokenize(query)
	for _, word := range words {
		if ids, exists := kg.nameIndex[word]; exists {
			for _, id := range ids {
				scores[id] += 10.0
			}
		}
	}

	// 2. 标签匹配
	for _, word := range words {
		if ids, exists := kg.tagIndex[word]; exists {
			for _, id := range ids {
				scores[id] += 5.0
			}
		}
	}

	// 3. 内容匹配 (简单子串)
	for id, entity := range kg.entities {
		if strings.Contains(strings.ToLower(entity.Content), query) {
			scores[id] += 2.0
		}
	}

	// 4. 访问频率加权
	for id, score := range scores {
		if entity, exists := kg.entities[id]; exists {
			scores[id] = score * (1.0 + float64(entity.AccessCount)*0.01)
		}
	}

	// 排序
	type scoredEntity struct {
		id    string
		score float64
	}
	var sorted []scoredEntity
	for id, score := range scores {
		sorted = append(sorted, scoredEntity{id: id, score: score})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	// 取前 limit 个
	if limit <= 0 {
		limit = 10
	}
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	var results []*SearchResult
	for _, se := range sorted {
		entity := kg.entities[se.id]
		results = append(results, &SearchResult{
			Entity: entity,
			Score:  se.score,
		})
	}

	return results, nil
}

// FindPath 查找两个实体之间的路径
func (kg *KnowledgeGraph) FindPath(ctx context.Context, fromID, toID string, maxDepth int) ([]string, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	if _, exists := kg.entities[fromID]; !exists {
		return nil, fmt.Errorf("entity not found: %s", fromID)
	}
	if _, exists := kg.entities[toID]; !exists {
		return nil, fmt.Errorf("entity not found: %s", toID)
	}

	// BFS
	type node struct {
		id    string
		path  []string
	}

	queue := []node{{id: fromID, path: []string{fromID}}}
	visited := map[string]bool{fromID: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if len(current.path) > maxDepth {
			continue
		}

		if current.id == toID {
			return current.path, nil
		}

		// 遍历邻居
		for _, relID := range kg.outEdges[current.id] {
			if rel, exists := kg.relations[relID]; exists {
				if !visited[rel.To] {
					visited[rel.To] = true
					newPath := make([]string, len(current.path)+1)
					copy(newPath, current.path)
					newPath[len(current.path)] = rel.To
					queue = append(queue, node{id: rel.To, path: newPath})
				}
			}
		}
	}

	return nil, fmt.Errorf("no path found between %s and %s", fromID, toID)
}

// GetStats 获取图谱统计
func (kg *KnowledgeGraph) GetStats(ctx context.Context) *GraphStats {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	stats := &GraphStats{
		EntityCount:    len(kg.entities),
		RelationCount:  len(kg.relations),
		TypeCounts:     make(map[EntityType]int),
		RelationCounts: make(map[RelationType]int),
		UpdatedAt:      time.Now(),
	}

	// 统计类型
	for _, entity := range kg.entities {
		stats.TypeCounts[entity.Type]++
	}

	// 统计关系类型
	for _, rel := range kg.relations {
		stats.RelationCounts[rel.Type]++
	}

	// Top entities (按访问次数)
	var entities []*Entity
	for _, e := range kg.entities {
		entities = append(entities, e)
	}
	sort.Slice(entities, func(i, j int) bool {
		return entities[i].AccessCount > entities[j].AccessCount
	})
	if len(entities) > 10 {
		entities = entities[:10]
	}
	stats.TopEntities = entities

	return stats
}

// ListEntities 列出实体
func (kg *KnowledgeGraph) ListEntities(ctx context.Context, entityType EntityType, offset, limit int) ([]*Entity, int, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	var ids []string
	if entityType != "" {
		ids = kg.typeIndex[entityType]
	} else {
		for id := range kg.entities {
			ids = append(ids, id)
		}
	}

	total := len(ids)

	if offset >= total {
		return nil, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	var result []*Entity
	for _, id := range ids[offset:end] {
		if entity, exists := kg.entities[id]; exists {
			result = append(result, entity)
		}
	}

	return result, total, nil
}

// Export 导出图谱数据
func (kg *KnowledgeGraph) Export(ctx context.Context) (map[string]interface{}, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	entities := make([]*Entity, 0, len(kg.entities))
	for _, e := range kg.entities {
		entities = append(entities, e)
	}

	relations := make([]*Relation, 0, len(kg.relations))
	for _, r := range kg.relations {
		relations = append(relations, r)
	}

	return map[string]interface{}{
		"entities":  entities,
		"relations": relations,
		"exported_at": time.Now(),
	}, nil
}

// Import 导入图谱数据
func (kg *KnowledgeGraph) Import(ctx context.Context, data map[string]interface{}) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// 简化实现: 实际应解析 JSON
	return fmt.Errorf("import not yet implemented")
}

// 辅助方法

func (kg *KnowledgeGraph) removeFromTypeIndex(entityType EntityType, id string) {
	ids := kg.typeIndex[entityType]
	for i, v := range ids {
		if v == id {
			kg.typeIndex[entityType] = append(ids[:i], ids[i+1:]...)
			return
		}
	}
}

func (kg *KnowledgeGraph) removeFromTagIndex(tag, id string) {
	ids := kg.tagIndex[tag]
	for i, v := range ids {
		if v == id {
			kg.tagIndex[tag] = append(ids[:i], ids[i+1:]...)
			return
		}
	}
}

func (kg *KnowledgeGraph) removeFromNameIndex(word, id string) {
	ids := kg.nameIndex[word]
	for i, v := range ids {
		if v == id {
			kg.nameIndex[word] = append(ids[:i], ids[i+1:]...)
			return
		}
	}
}

// tokenize 简单分词
func tokenize(text string) []string {
	// 简单分词: 按空格和标点分割
	// 实际应使用中文分词库
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '!' || r == '?' ||
			r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}' ||
			r == ':' || r == ';' || r == '"' || r == '\'' || r == '-' || r == '_'
	})
	return words
}
