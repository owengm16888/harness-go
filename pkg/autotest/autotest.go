package autotest

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestSuite 测试套件
type TestSuite struct {
	Name        string
	Description string
	Tests       []*TestCase
	Setup       func(ctx context.Context) error
	Teardown    func(ctx context.Context) error
	BeforeEach  func(ctx context.Context, test *TestCase) error
	AfterEach   func(ctx context.Context, test *TestCase) error
}

// TestCase 测试用例
type TestCase struct {
	Name        string
	Description string
	Tags        []string
	Timeout     time.Duration
	Retry       int
	Skip        bool
	SkipReason  string
	TestFunc    func(ctx context.Context) error
	Assertions  []Assertion
}

// Assertion 断言
type Assertion struct {
	Name     string
	Actual   interface{}
	Expected interface{}
	Type     AssertionType
}

// AssertionType 断言类型
type AssertionType string

const (
	AssertEqual       AssertionType = "equal"
	AssertNotEqual    AssertionType = "not_equal"
	AssertNil         AssertionType = "nil"
	AssertNotNil      AssertionType = "not_nil"
	AssertTrue        AssertionType = "true"
	AssertFalse       AssertionType = "false"
	AssertContains    AssertionType = "contains"
	AssertNotContains AssertionType = "not_contains"
	AssertLen         AssertionType = "len"
	AssertGreater     AssertionType = "greater"
	AssertLess        AssertionType = "less"
)

// TestResult 测试结果
type TestResult struct {
	SuiteName   string        `json:"suite_name"`
	TestName    string        `json:"test_name"`
	Status      TestStatus    `json:"status"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Assertions  int           `json:"assertions"`
	Passed      int           `json:"passed"`
	Failed      int           `json:"failed"`
	RetryCount  int           `json:"retry_count"`
	Timestamp   time.Time     `json:"timestamp"`
}

// TestStatus 测试状态
type TestStatus string

const (
	StatusPassed  TestStatus = "passed"
	StatusFailed  TestStatus = "failed"
	StatusSkipped TestStatus = "skipped"
	StatusError   TestStatus = "error"
)

// SuiteResult 套件结果
type SuiteResult struct {
	SuiteName    string        `json:"suite_name"`
	TotalTests   int           `json:"total_tests"`
	Passed       int           `json:"passed"`
	Failed       int           `json:"failed"`
	Skipped      int           `json:"skipped"`
	Errors       int           `json:"errors"`
	Duration     time.Duration `json:"duration"`
	Results      []*TestResult `json:"results"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
}

// TestRunner 测试运行器
type TestRunner struct {
	suites []*TestSuite
}

// NewTestRunner 创建测试运行器
func NewTestRunner() *TestRunner {
	return &TestRunner{}
}

// AddSuite 添加测试套件
func (r *TestRunner) AddSuite(suite *TestSuite) {
	r.suites = append(r.suites, suite)
}

// Run 运行所有测试
func (r *TestRunner) Run(ctx context.Context) []*SuiteResult {
	var results []*SuiteResult

	for _, suite := range r.suites {
		result := r.runSuite(ctx, suite)
		results = append(results, result)
	}

	return results
}

// runSuite 运行测试套件
func (r *TestRunner) runSuite(ctx context.Context, suite *TestSuite) *SuiteResult {
	result := &SuiteResult{
		SuiteName: suite.Name,
		StartTime: time.Now(),
	}

	// Setup
	if suite.Setup != nil {
		if err := suite.Setup(ctx); err != nil {
			result.Errors++
			result.Results = append(result.Results, &TestResult{
				SuiteName: suite.Name,
				TestName:  "setup",
				Status:    StatusError,
				Error:     err.Error(),
				Timestamp: time.Now(),
			})
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		}
	}

	// 运行测试
	for _, test := range suite.Tests {
		// BeforeEach
		if suite.BeforeEach != nil {
			if err := suite.BeforeEach(ctx, test); err != nil {
				result.Errors++
				result.Results = append(result.Results, &TestResult{
					SuiteName: suite.Name,
					TestName:  test.Name,
					Status:    StatusError,
					Error:     err.Error(),
					Timestamp: time.Now(),
				})
				continue
			}
		}

		// 运行测试
		testResult := r.runTest(ctx, suite.Name, test)
		result.Results = append(result.Results, testResult)
		result.TotalTests++

		switch testResult.Status {
		case StatusPassed:
			result.Passed++
		case StatusFailed:
			result.Failed++
		case StatusSkipped:
			result.Skipped++
		case StatusError:
			result.Errors++
		}

		// AfterEach
		if suite.AfterEach != nil {
			if err := suite.AfterEach(ctx, test); err != nil {
				// 记录但不覆盖测试结果
			}
		}
	}

	// Teardown
	if suite.Teardown != nil {
		if err := suite.Teardown(ctx); err != nil {
			// 记录但不覆盖测试结果
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// runTest 运行单个测试
func (r *TestRunner) runTest(ctx context.Context, suiteName string, test *TestCase) *TestResult {
	result := &TestResult{
		SuiteName: suiteName,
		TestName:  test.Name,
		Timestamp: time.Now(),
	}

	// 检查是否跳过
	if test.Skip {
		result.Status = StatusSkipped
		return result
	}

	// 创建带超时的上下文
	testCtx := ctx
	if test.Timeout > 0 {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(ctx, test.Timeout)
		defer cancel()
	}

	// 重试机制
	maxRetry := test.Retry
	if maxRetry < 0 {
		maxRetry = 0
	}

	start := time.Now()
	var err error

	for retry := 0; retry <= maxRetry; retry++ {
		result.RetryCount = retry

		// 运行测试函数
		if test.TestFunc != nil {
			err = test.TestFunc(testCtx)
			if err == nil {
				break
			}
		}

		// 运行断言
		for _, assertion := range test.Assertions {
			if !r.evaluateAssertion(assertion) {
				err = fmt.Errorf("assertion failed: %s", assertion.Name)
				break
			}
			result.Passed++
		}

		if err == nil {
			break
		}

		// 如果有重试，等待一下
		if retry < maxRetry {
			time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
		}
	}

	result.Duration = time.Since(start)
	result.Assertions = len(test.Assertions)

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		result.Failed = result.Assertions - result.Passed
	} else {
		result.Status = StatusPassed
	}

	return result
}

// evaluateAssertion 评估断言
func (r *TestRunner) evaluateAssertion(assertion Assertion) bool {
	switch assertion.Type {
	case AssertEqual:
		return reflect.DeepEqual(assertion.Actual, assertion.Expected)
	case AssertNotEqual:
		return !reflect.DeepEqual(assertion.Actual, assertion.Expected)
	case AssertNil:
		return assertion.Actual == nil
	case AssertNotNil:
		return assertion.Actual != nil
	case AssertTrue:
		v, ok := assertion.Actual.(bool)
		return ok && v
	case AssertFalse:
		v, ok := assertion.Actual.(bool)
		return ok && !v
	case AssertContains:
		return contains(assertion.Actual, assertion.Expected)
	case AssertNotContains:
		return !contains(assertion.Actual, assertion.Expected)
	case AssertLen:
		return lengthEquals(assertion.Actual, assertion.Expected)
	case AssertGreater:
		return greaterThan(assertion.Actual, assertion.Expected)
	case AssertLess:
		return lessThan(assertion.Actual, assertion.Expected)
	default:
		return false
	}
}

// 辅助断言函数

func contains(actual, expected interface{}) bool {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	return strings.Contains(actualStr, expectedStr)
}

func lengthEquals(actual, expected interface{}) bool {
	v := reflect.ValueOf(actual)
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array || v.Kind() == reflect.String {
		expectedLen, ok := expected.(int)
		if !ok {
			return false
		}
		return v.Len() == expectedLen
	}
	return false
}

func greaterThan(actual, expected interface{}) bool {
	return compareValues(actual, expected) > 0
}

func lessThan(actual, expected interface{}) bool {
	return compareValues(actual, expected) < 0
}

func compareValues(actual, expected interface{}) int {
	switch a := actual.(type) {
	case int:
		if b, ok := expected.(int); ok {
			if a > b {
				return 1
			} else if a < b {
				return -1
			}
			return 0
		}
	case float64:
		if b, ok := expected.(float64); ok {
			if a > b {
				return 1
			} else if a < b {
				return -1
			}
			return 0
		}
	case string:
		if b, ok := expected.(string); ok {
			return strings.Compare(a, b)
		}
	}
	return 0
}

// PrintResults 打印测试结果
func PrintResults(results []*SuiteResult) {
	totalTests := 0
	totalPassed := 0
	totalFailed := 0
	totalSkipped := 0
	totalErrors := 0
	totalDuration := time.Duration(0)

	for _, suite := range results {
		fmt.Printf("\n=== %s ===\n", suite.SuiteName)
		fmt.Printf("Tests: %d | Passed: %d | Failed: %d | Skipped: %d | Errors: %d\n",
			suite.TotalTests, suite.Passed, suite.Failed, suite.Skipped, suite.Errors)
		fmt.Printf("Duration: %v\n", suite.Duration)

		for _, result := range suite.Results {
			status := "✓"
			if result.Status == StatusFailed {
				status = "✗"
			} else if result.Status == StatusSkipped {
				status = "○"
			} else if result.Status == StatusError {
				status = "✗"
			}

			fmt.Printf("  %s %s (%v)", status, result.TestName, result.Duration)
			if result.RetryCount > 0 {
				fmt.Printf(" [retry %d]", result.RetryCount)
			}
			if result.Error != "" {
				fmt.Printf(" - %s", result.Error)
			}
			fmt.Println()
		}

		totalTests += suite.TotalTests
		totalPassed += suite.Passed
		totalFailed += suite.Failed
		totalSkipped += suite.Skipped
		totalErrors += suite.Errors
		totalDuration += suite.Duration
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total: %d | Passed: %d | Failed: %d | Skipped: %d | Errors: %d\n",
		totalTests, totalPassed, totalFailed, totalSkipped, totalErrors)
	fmt.Printf("Duration: %v\n", totalDuration)

	if totalFailed == 0 && totalErrors == 0 {
		fmt.Println("✓ All tests passed!")
	} else {
		fmt.Println("✗ Some tests failed!")
	}
}

// BenchmarkHelper 基准测试辅助
type BenchmarkHelper struct {
	name string
}

// NewBenchmarkHelper 创建基准测试辅助
func NewBenchmarkHelper(name string) *BenchmarkHelper {
	return &BenchmarkHelper{name: name}
}

// Run 运行基准测试
func (h *BenchmarkHelper) Run(b *testing.B, fn func(b *testing.B)) {
	b.Run(h.name, fn)
}

// Measure 测量函数执行时间
func Measure(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// MeasureWithResult 测量函数执行时间并返回结果
func MeasureWithResult(fn func() interface{}) (interface{}, time.Duration) {
	start := time.Now()
	result := fn()
	return result, time.Since(start)
}

// MemoryStats 内存统计
type MemoryStats struct {
	Alloc        uint64 `json:"alloc"`
	TotalAlloc   uint64 `json:"total_alloc"`
	Sys          uint64 `json:"sys"`
	NumGC        uint32 `json:"num_gc"`
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapObjects  uint64 `json:"heap_objects"`
	StackInuse   uint64 `json:"stack_inuse"`
	NumGoroutine int    `json:"num_goroutine"`
}

// GetMemoryStats 获取内存统计
func GetMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Alloc:        m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		Sys:          m.Sys,
		NumGC:        m.NumGC,
		HeapAlloc:    m.HeapAlloc,
		HeapInuse:    m.HeapInuse,
		HeapObjects:  m.HeapObjects,
		StackInuse:   m.StackInuse,
		NumGoroutine: runtime.NumGoroutine(),
	}
}

// Assert 断言辅助函数
type Assert struct {
	t *testing.T
}

// NewAssert 创建断言辅助
func NewAssert(t *testing.T) *Assert {
	return &Assert{t: t}
}

// Equal 等于
func (a *Assert) Equal(actual, expected interface{}) {
	a.t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		a.t.Errorf("expected %v, got %v", expected, actual)
	}
}

// NotEqual 不等于
func (a *Assert) NotEqual(actual, expected interface{}) {
	a.t.Helper()
	if reflect.DeepEqual(actual, expected) {
		a.t.Errorf("expected not equal to %v", expected)
	}
}

// Nil 为 nil
func (a *Assert) Nil(actual interface{}) {
	a.t.Helper()
	if actual != nil {
		a.t.Errorf("expected nil, got %v", actual)
	}
}

// NotNil 不为 nil
func (a *Assert) NotNil(actual interface{}) {
	a.t.Helper()
	if actual == nil {
		a.t.Errorf("expected not nil")
	}
}

// True 为 true
func (a *Assert) True(actual bool) {
	a.t.Helper()
	if !actual {
		a.t.Errorf("expected true, got false")
	}
}

// False 为 false
func (a *Assert) False(actual bool) {
	a.t.Helper()
	if actual {
		a.t.Errorf("expected false, got true")
	}
}

// Contains 包含
func (a *Assert) Contains(actual, expected interface{}) {
	a.t.Helper()
	if !contains(actual, expected) {
		a.t.Errorf("expected %v to contain %v", actual, expected)
	}
}

// Len 长度
func (a *Assert) Len(actual interface{}, expected int) {
	a.t.Helper()
	if !lengthEquals(actual, expected) {
		v := reflect.ValueOf(actual)
		a.t.Errorf("expected length %d, got %d", expected, v.Len())
	}
}

// NoError 无错误
func (a *Assert) NoError(err error) {
	a.t.Helper()
	if err != nil {
		a.t.Errorf("expected no error, got %v", err)
	}
}

// HasError 有错误
func (a *Assert) HasError(err error) {
	a.t.Helper()
	if err == nil {
		a.t.Errorf("expected error, got nil")
	}
}

// ErrorContains 错误包含
func (a *Assert) ErrorContains(err error, expected string) {
	a.t.Helper()
	if err == nil {
		a.t.Errorf("expected error, got nil")
		return
	}
	if !strings.Contains(err.Error(), expected) {
		a.t.Errorf("expected error to contain %q, got %q", expected, err.Error())
	}
}
