package adapters

// Adapter 接口定义在 internal/core/engine.go 中 (core.Adapter)
// 所有适配器 (ClaudeCodeAdapter, HermesAdapter, CodexCLIAdapter)
// 通过 Go 结构化类型隐式实现该接口，无需显式声明。
//
// 接口方法:
//   Name() string
//   Initialize(ctx context.Context, config config.AdapterConfig) error
//   ExecuteTask(ctx context.Context, task models.Task) (models.Result, error)
//   GetState(ctx context.Context) (models.State, error)
//   Cleanup(ctx context.Context) error
