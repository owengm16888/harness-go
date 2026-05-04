package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/harness-engineering/harness/internal/storage"
	"github.com/harness-engineering/harness/pkg/cache"
	"github.com/harness-engineering/harness/models"
)

// Indexer TF-IDF 加权索引器
type Indexer struct {
	// term -> entryID -> term frequency
	termFreq map[string]map[string]int
	// entryID -> total term count
	entryLen map[string]int
	// term -> document frequency (多少条目包含该 term)
	docFreq map[string]int
	// 总条目数
	totalDocs int
}

// NewIndexer 创建索引器
func NewIndexer() *Indexer {
	return &Indexer{
		termFreq: make(map[string]map[string]int),
		entryLen: make(map[string]int),
		docFreq:  make(map[string]int),
	}
}

// stopwords 停用词表（高频低信息量词汇）
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "about": true, "between": true, "through": true,
	"and": true, "or": true, "but": true, "not": true, "no": true,
	"it": true, "its": true, "this": true, "that": true, "these": true,
	"i": true, "we": true, "you": true, "he": true, "she": true,
	"they": true, "my": true, "your": true, "his": true, "her": true,
	"how": true, "what": true, "when": true, "where": true, "which": true,
	"who": true, "whom": true, "why": true,
	"的": true, "了": true, "在": true, "是": true, "我": true,
	"有": true, "和": true, "就": true, "不": true, "人": true,
	"都": true, "一": true, "一个": true, "上": true, "也": true,
	"很": true, "到": true, "说": true, "要": true, "去": true,
	"你": true, "会": true, "着": true, "没有": true, "看": true,
	"好": true, "自己": true, "这": true, "他": true, "她": true,
	"它": true, "们": true, "那": true, "些": true, "什么": true,
}

// tokenize 分词（支持英文 + CJK 字符，过滤停用词）
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	current := ""

	for _, r := range text {
		isWord := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		isCJK := r >= 0x4e00 && r <= 0x9fff

		if isWord {
			current += string(r)
		} else if isCJK {
			// CJK 字符：每个字符单独成词（bigram 也可）
			if current != "" {
				if !stopwords[current] {
					tokens = append(tokens, current)
				}
				current = ""
			}
			char := string(r)
			if !stopwords[char] {
				tokens = append(tokens, char)
			}
		} else {
			if current != "" {
				if !stopwords[current] {
					tokens = append(tokens, current)
				}
				current = ""
			}
		}
	}
	if current != "" && !stopwords[current] {
		tokens = append(tokens, current)
	}

	return tokens
}

// Index 索引知识条目
func (idx *Indexer) Index(ctx context.Context, entry *models.KnowledgeEntry) error {
	// 合并标题 + 内容 + 标签为文本
	text := entry.Title + " " + entry.Content + " " + strings.Join(entry.Tags, " ")
	tokens := tokenize(text)

	// 计算词频
	tf := make(map[string]int)
	for _, token := range tokens {
		tf[token]++
	}

	// 更新索引
	idx.termFreq[entry.ID] = tf
	idx.entryLen[entry.ID] = len(tokens)
	idx.totalDocs++

	// 更新文档频率
	for term := range tf {
		idx.docFreq[term]++
	}

	return nil
}

// Remove 移除索引
func (idx *Indexer) Remove(ctx context.Context, entry *models.KnowledgeEntry) error {
	if tf, exists := idx.termFreq[entry.ID]; exists {
		for term := range tf {
			idx.docFreq[term]--
			if idx.docFreq[term] <= 0 {
				delete(idx.docFreq, term)
			}
		}
		delete(idx.termFreq, entry.ID)
		delete(idx.entryLen, entry.ID)
		idx.totalDocs--
	}
	return nil
}

// Search 搜索（TF-IDF 评分）
func (idx *Indexer) Search(ctx context.Context, query string, limit int) ([]string, error) {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil, nil
	}

	// 计算每个条目的 TF-IDF 得分
	scores := make(map[string]float64)

	for _, term := range queryTokens {
		// 收集包含该 term 的所有条目
		matchingEntries := make(map[string]int) // entryID → tf

		for entryID, tfMap := range idx.termFreq {
			if tf, exists := tfMap[term]; exists {
				matchingEntries[entryID] = tf
			} else {
				// 子串匹配: term 包含 indexTerm 或 indexTerm 包含 term
				for indexTerm, tf := range tfMap {
					if strings.Contains(indexTerm, term) || strings.Contains(term, indexTerm) {
						matchingEntries[entryID] += tf
					}
				}
			}
		}

		if len(matchingEntries) == 0 {
			continue
		}

		// IDF = log(N / df)
		df := float64(len(matchingEntries))
		if df == 0 {
			df = 1
		}
		idf := math.Log(float64(idx.totalDocs) / df)

		for entryID, tf := range matchingEntries {
			// TF = 1 + log(tf)
			tfScore := 1.0 + math.Log(float64(tf))
			scores[entryID] += tfScore * idf
		}
	}

	// 按分数排序
	type scoredEntry struct {
		id    string
		score float64
	}
	var sorted []scoredEntry
	for id, score := range scores {
		sorted = append(sorted, scoredEntry{id, score})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	// 取 top-N
	var results []string
	for i, se := range sorted {
		if i >= limit {
			break
		}
		results = append(results, se.id)
	}

	return results, nil
}

// KnowledgeBase 知识库（带 LRU 热点缓存）
type KnowledgeBase struct {
	mu      sync.RWMutex
	entries map[string]*models.KnowledgeEntry
	store   storage.KnowledgeStore
	indexer *Indexer
	cache   *cache.MemoryCache // 搜索结果缓存
}

// NewKnowledgeBase 创建知识库（带 LRU 缓存）
func NewKnowledgeBase(store storage.KnowledgeStore, indexer *Indexer) *KnowledgeBase {
	return &KnowledgeBase{
		entries: make(map[string]*models.KnowledgeEntry),
		store:   store,
		indexer: indexer,
		cache:   cache.NewMemoryCache(500, 2*time.Minute),
	}
}

// LoadFromStorage 从持久化存储加载所有知识条目到内存
func (kb *KnowledgeBase) LoadFromStorage(ctx context.Context) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	offset := 0
	limit := 100
	for {
		entries, err := kb.store.ListKnowledge(ctx, offset, limit)
		if err != nil {
			return fmt.Errorf("failed to load knowledge from storage: %w", err)
		}
		if len(entries) == 0 {
			break
		}

		for _, entry := range entries {
			kb.entries[entry.ID] = entry
			kb.indexer.Index(ctx, entry)
		}

		offset += len(entries)
		if len(entries) < limit {
			break
		}
	}

	return nil
}

// AddEntry 添加知识条目
func (kb *KnowledgeBase) AddEntry(ctx context.Context, entry models.KnowledgeEntry) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	// 设置时间戳
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	// 保存到存储
	if err := kb.store.SaveKnowledge(ctx, &entry); err != nil {
		return fmt.Errorf("failed to save knowledge entry: %w", err)
	}

	// 添加到内存
	kb.entries[entry.ID] = &entry

	// 更新索引
	if err := kb.indexer.Index(ctx, &entry); err != nil {
		return fmt.Errorf("failed to index knowledge entry: %w", err)
	}

	return nil
}

// Search 搜索知识（带 LRU 缓存）
func (kb *KnowledgeBase) Search(ctx context.Context, query string, limit int) ([]*models.KnowledgeEntry, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("search:%s:%d", query, limit)
	if cached, ok := kb.cache.Get(cacheKey); ok {
		return cached.([]*models.KnowledgeEntry), nil
	}

	kb.mu.RLock()
	defer kb.mu.RUnlock()

	// 使用索引搜索
	ids, err := kb.indexer.Search(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 获取条目
	var entries []*models.KnowledgeEntry
	for _, id := range ids {
		if entry, exists := kb.entries[id]; exists {
			entry.AccessCount++
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// GetEntry 获取知识条目
func (kb *KnowledgeBase) GetEntry(ctx context.Context, id string) (*models.KnowledgeEntry, error) {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	entry, exists := kb.entries[id]
	if !exists {
		return nil, fmt.Errorf("knowledge entry not found: %s", id)
	}

	entry.AccessCount++
	return entry, nil
}

// UpdateEntry 更新知识条目
func (kb *KnowledgeBase) UpdateEntry(ctx context.Context, id string, update models.KnowledgeUpdate) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	entry, exists := kb.entries[id]
	if !exists {
		return fmt.Errorf("knowledge entry not found: %s", id)
	}

	// 应用更新
	if update.Title != "" {
		entry.Title = update.Title
	}
	if update.Content != "" {
		entry.Content = update.Content
	}
	if update.Tags != nil {
		entry.Tags = update.Tags
	}
	if update.Metadata != nil {
		for k, v := range update.Metadata {
			entry.Metadata[k] = v
		}
	}
	entry.UpdatedAt = time.Now()

	// 保存到存储
	if err := kb.store.SaveKnowledge(ctx, entry); err != nil {
		return fmt.Errorf("failed to save knowledge entry: %w", err)
	}

	// 更新索引
	if err := kb.indexer.Index(ctx, entry); err != nil {
		return fmt.Errorf("failed to index knowledge entry: %w", err)
	}

	return nil
}

// DeleteEntry 删除知识条目
func (kb *KnowledgeBase) DeleteEntry(ctx context.Context, id string) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	entry, exists := kb.entries[id]
	if !exists {
		return fmt.Errorf("knowledge entry not found: %s", id)
	}

	// 从存储删除
	if err := kb.store.DeleteKnowledge(ctx, id); err != nil {
		return fmt.Errorf("failed to delete knowledge entry: %w", err)
	}

	// 从内存删除
	delete(kb.entries, id)

	// 从索引删除
	if err := kb.indexer.Remove(ctx, entry); err != nil {
		return fmt.Errorf("failed to remove from index: %w", err)
	}

	return nil
}

// ListEntries 列出知识条目
func (kb *KnowledgeBase) ListEntries(ctx context.Context, offset, limit int) ([]*models.KnowledgeEntry, error) {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	var entries []*models.KnowledgeEntry
	for _, entry := range kb.entries {
		entries = append(entries, entry)
	}

	// 分页 — copy 出去避免子切片泄露 (面试知识点: 切片陷阱)
	if offset >= len(entries) {
		return []*models.KnowledgeEntry{}, nil
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}

	result := make([]*models.KnowledgeEntry, end-offset)
	copy(result, entries[offset:end])
	return result, nil
}
