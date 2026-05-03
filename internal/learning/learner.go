package learning

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/harness-engineering/harness/models"
)

// Learner 学习器 — 从任务执行观察中学习模式
type Learner struct {
	mu           sync.RWMutex
	observations []models.Observation
	patterns     map[string]*LearnedPattern
	threshold    float64
	minSamples   int
}

// LearnedPattern 学习到的模式
type LearnedPattern struct {
	ID          string         `json:"id"`
	Features    []Feature      `json:"features"`
	Outcome     models.Outcome `json:"outcome"`
	Confidence  float64        `json:"confidence"`
	SampleCount int            `json:"sample_count"`
	SuccessCount int           `json:"success_count"`
	LastUpdated time.Time      `json:"last_updated"`
}

// Feature 特征
type Feature struct {
	Name   string      `json:"name"`
	Value  interface{} `json:"value"`
	Weight float64     `json:"weight"`
}

// NewLearner 创建学习器
func NewLearner(minSamples int, threshold float64) *Learner {
	return &Learner{
		observations: []models.Observation{},
		patterns:     make(map[string]*LearnedPattern),
		threshold:    threshold,
		minSamples:   minSamples,
	}
}

// Observe 观察任务执行结果
func (l *Learner) Observe(ctx context.Context, obs models.Observation) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 添加观察
	l.observations = append(l.observations, obs)

	// 提取特征
	features := l.extractFeatures(obs)

	// 查找或创建模式
	patternID := l.findPattern(features)
	if patternID == "" {
		patternID = l.createPattern(features, obs)
	}

	// 更新模式
	l.updatePattern(patternID, obs)

	return nil
}

// Predict 预测任务结果
func (l *Learner) Predict(ctx context.Context, task models.Task) (*models.Prediction, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 提取特征
	features := l.extractFeaturesFromTask(task)

	// 查找匹配的模式
	pattern := l.findBestMatch(features)
	if pattern == nil {
		return &models.Prediction{
			Confidence:     0,
			Recommendation: "no pattern found",
		}, nil
	}

	return &models.Prediction{
		PatternID:       pattern.ID,
		Confidence:      pattern.Confidence,
		ExpectedOutcome: pattern.Outcome,
		Recommendation:  l.generateRecommendation(pattern),
	}, nil
}

// extractFeatures 从观察中提取特征
func (l *Learner) extractFeatures(obs models.Observation) []Feature {
	var features []Feature

	// 任务类型特征
	features = append(features, Feature{
		Name:   "task_type",
		Value:  obs.Task.Type,
		Weight: 1.0,
	})

	// 环境特征
	features = append(features, Feature{
		Name:   "environment",
		Value:  obs.Task.Context["environment"],
		Weight: 0.8,
	})

	// 复杂度特征
	complexity := l.calculateComplexity(obs.Task)
	features = append(features, Feature{
		Name:   "complexity",
		Value:  complexity,
		Weight: 0.6,
	})

	return features
}

// extractFeaturesFromTask 从任务中提取特征
func (l *Learner) extractFeaturesFromTask(task models.Task) []Feature {
	var features []Feature

	features = append(features, Feature{
		Name:   "task_type",
		Value:  task.Type,
		Weight: 1.0,
	})

	features = append(features, Feature{
		Name:   "environment",
		Value:  task.Context["environment"],
		Weight: 0.8,
	})

	complexity := l.calculateComplexity(task)
	features = append(features, Feature{
		Name:   "complexity",
		Value:  complexity,
		Weight: 0.6,
	})

	return features
}

// findPattern 查找与特征匹配的已有模式
func (l *Learner) findPattern(features []Feature) string {
	bestMatch := ""
	bestScore := 0.0

	for id, pattern := range l.patterns {
		score := l.calculateSimilarity(features, pattern.Features)
		if score > bestScore && score >= l.threshold {
			bestScore = score
			bestMatch = id
		}
	}

	return bestMatch
}

// createPattern 创建新模式
func (l *Learner) createPattern(features []Feature, obs models.Observation) string {
	id := generateID()
	l.patterns[id] = &LearnedPattern{
		ID:           id,
		Features:     features,
		Outcome:      l.observationToOutcome(obs),
		Confidence:   0.5,
		SampleCount:  1,
		SuccessCount: boolToInt(obs.Success),
		LastUpdated:  time.Now(),
	}
	return id
}

// updatePattern 更新已有模式
func (l *Learner) updatePattern(patternID string, obs models.Observation) {
	pattern, exists := l.patterns[patternID]
	if !exists {
		return
	}

	// 更新样本数
	pattern.SampleCount++
	if obs.Success {
		pattern.SuccessCount++
	}

	// 更新结果（增量移动平均）
	outcome := l.observationToOutcome(obs)
	n := float64(pattern.SampleCount)
	pattern.Outcome.Duration = (pattern.Outcome.Duration*(n-1) + outcome.Duration) / n
	pattern.Outcome.Quality = (pattern.Outcome.Quality*(n-1) + outcome.Quality) / n
	pattern.Outcome.Success = float64(pattern.SuccessCount) / n

	// 更新置信度
	pattern.Confidence = l.calculateConfidence(pattern)

	// 更新时间
	pattern.LastUpdated = time.Now()
}

// calculateSimilarity 计算两组特征的加权相似度
func (l *Learner) calculateSimilarity(features1, features2 []Feature) float64 {
	if len(features1) != len(features2) {
		return 0
	}

	totalWeight := 0.0
	matchWeight := 0.0

	for i, f1 := range features1 {
		f2 := features2[i]
		totalWeight += f1.Weight + f2.Weight

		if f1.Name == f2.Name && f1.Value == f2.Value {
			matchWeight += f1.Weight + f2.Weight
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return matchWeight / totalWeight
}

// calculateConfidence 使用 Beta-Binomial 共轭模型计算置信度
//
// 先验: Beta(1, 1) — 均匀分布（无信息先验）
// 数据: k 次成功，n 次试验
// 后验: Beta(1+k, 1+n-k)
// 后验均值: (1+k) / (2+n)
//
// 当样本量不足时返回 0.5（先验均值）
func (l *Learner) calculateConfidence(pattern *LearnedPattern) float64 {
	if pattern.SampleCount < l.minSamples {
		return 0.5
	}

	k := float64(pattern.SuccessCount)
	n := float64(pattern.SampleCount)

	// Beta(1,1) 先验 → Beta(1+k, 1+n-k) 后验
	// 后验均值
	posteriorMean := (1.0 + k) / (2.0 + n)

	return posteriorMean
}

// calculateComplexity 计算任务复杂度 (0.0 ~ 1.0)
func (l *Learner) calculateComplexity(task models.Task) float64 {
	complexity := 0.0

	// 基于描述长度
	complexity += float64(len(task.Description)) / 1000.0

	// 基于约束数量
	complexity += float64(len(task.Constraints)) * 0.1

	// 基于上下文大小
	complexity += float64(len(task.Context)) * 0.05

	return math.Min(complexity, 1.0)
}

// observationToOutcome 观察转结果
func (l *Learner) observationToOutcome(obs models.Observation) models.Outcome {
	return models.Outcome{
		Success:  boolToFloat(obs.Success),
		Duration: obs.Result.Metrics.Duration.Seconds(),
		Quality:  l.calculateQuality(obs.Result),
	}
}

// calculateQuality 计算结果质量 (0.0 ~ 1.0)
func (l *Learner) calculateQuality(result models.Result) float64 {
	quality := 1.0

	// 基于错误数
	quality -= float64(len(result.Errors)) * 0.1

	// 基于重试次数
	quality -= float64(result.Metrics.RetryCount) * 0.05

	return math.Max(quality, 0)
}

// findBestMatch 查找最佳匹配模式
func (l *Learner) findBestMatch(features []Feature) *LearnedPattern {
	bestPattern := (*LearnedPattern)(nil)
	bestScore := 0.0

	for _, pattern := range l.patterns {
		score := l.calculateSimilarity(features, pattern.Features)
		if score > bestScore {
			bestScore = score
			bestPattern = pattern
		}
	}

	return bestPattern
}

// generateRecommendation 生成建议
func (l *Learner) generateRecommendation(pattern *LearnedPattern) string {
	if pattern.Confidence > 0.8 {
		return "pattern has high success rate, proceed with confidence"
	} else if pattern.Confidence > 0.5 {
		return "pattern has moderate success rate, proceed with caution"
	} else {
		return "pattern has low success rate, consider alternative approach"
	}
}

// --- 辅助函数 ---

// boolToFloat 布尔转浮点
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// boolToInt 布尔转整数
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// generateID 生成不重复的 ID（crypto/rand + 时间戳）
func generateID() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%s-%06d", time.Now().Format("20060102150405.000000000"), n.Int64())
}
