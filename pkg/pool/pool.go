package pool

import (
	"context"
	"sync"
	"time"
)

// Pool 连接池接口
type Pool interface {
	Get(ctx context.Context) (interface{}, error)
	Put(ctx context.Context, conn interface{}) error
	Close() error
	Size() int
	Available() int
}

// PoolConfig 连接池配置
type PoolConfig struct {
	MaxSize     int           `yaml:"max_size"`
	MinSize     int           `yaml:"min_size"`
	MaxIdle     int           `yaml:"max_idle"`
	MaxLifetime time.Duration `yaml:"max_lifetime"`
	Timeout     time.Duration `yaml:"timeout"`
}

// GenericPool 通用连接池
type GenericPool struct {
	mu          sync.RWMutex
	factory     func(ctx context.Context) (interface{}, error)
	close       func(conn interface{}) error
	validate    func(conn interface{}) bool
	conns       chan *poolConn
	maxSize     int
	minSize     int
	maxIdle     int
	maxLifetime time.Duration
	timeout     time.Duration
	closed      bool
}

// poolConn 连接池连接
type poolConn struct {
	conn      interface{}
	createdAt time.Time
	lastUsed  time.Time
}

// NewGenericPool 创建通用连接池
func NewGenericPool(
	factory func(ctx context.Context) (interface{}, error),
	close func(conn interface{}) error,
	validate func(conn interface{}) bool,
	cfg PoolConfig,
) (*GenericPool, error) {
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 10
	}
	if cfg.MinSize == 0 {
		cfg.MinSize = 1
	}
	if cfg.MaxIdle == 0 {
		cfg.MaxIdle = 5
	}
	if cfg.MaxLifetime == 0 {
		cfg.MaxLifetime = 1 * time.Hour
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	pool := &GenericPool{
		factory:     factory,
		close:       close,
		validate:    validate,
		conns:       make(chan *poolConn, cfg.MaxSize),
		maxSize:     cfg.MaxSize,
		minSize:     cfg.MinSize,
		maxIdle:     cfg.MaxIdle,
		maxLifetime: cfg.MaxLifetime,
		timeout:     cfg.Timeout,
	}

	// 创建最小连接数
	for i := 0; i < cfg.MinSize; i++ {
		conn, err := factory(context.Background())
		if err != nil {
			return nil, err
		}
		pool.conns <- &poolConn{
			conn:      conn,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
		}
	}

	// 启动清理协程
	go pool.cleanup()

	return pool, nil
}

// Get 获取连接
func (p *GenericPool) Get(ctx context.Context) (interface{}, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrPoolClosed
	}
	p.mu.RUnlock()

	// 尝试从池中获取连接
	select {
	case pc := <-p.conns:
		// 检查连接是否有效
		if p.validate != nil && !p.validate(pc.conn) {
			p.close(pc.conn)
			return p.createConn(ctx)
		}

		// 检查连接是否过期
		if time.Since(pc.createdAt) > p.maxLifetime {
			p.close(pc.conn)
			return p.createConn(ctx)
		}

		pc.lastUsed = time.Now()
		return pc.conn, nil
	default:
		// 池中没有可用连接，创建新连接
		return p.createConn(ctx)
	}
}

// Put 归还连接
func (p *GenericPool) Put(ctx context.Context, conn interface{}) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		if p.close != nil {
			return p.close(conn)
		}
		return nil
	}
	p.mu.RUnlock()

	// 检查池是否已满
	if len(p.conns) >= p.maxIdle {
		if p.close != nil {
			return p.close(conn)
		}
		return nil
	}

	pc := &poolConn{
		conn:      conn,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}

	select {
	case p.conns <- pc:
		return nil
	default:
		// 池已满，关闭连接
		if p.close != nil {
			return p.close(conn)
		}
		return nil
	}
}

// Close 关闭连接池
func (p *GenericPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.conns)

	// 关闭所有连接
	for pc := range p.conns {
		if p.close != nil {
			p.close(pc.conn)
		}
	}

	return nil
}

// Size 获取池大小
func (p *GenericPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.conns)
}

// Available 获取可用连接数
func (p *GenericPool) Available() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.conns)
}

// createConn 创建新连接
func (p *GenericPool) createConn(ctx context.Context) (interface{}, error) {
	// 检查是否超过最大连接数
	if p.Size() >= p.maxSize {
		return nil, ErrPoolFull
	}

	conn, err := p.factory(ctx)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// cleanup 清理过期连接
func (p *GenericPool) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.RLock()
		if p.closed {
			p.mu.RUnlock()
			return
		}
		p.mu.RUnlock()

		now := time.Now()
		conns := make([]*poolConn, 0, len(p.conns))

		// 收集所有连接
		for {
			select {
			case pc := <-p.conns:
				conns = append(conns, pc)
			default:
				goto done
			}
		}
	done:

		// 清理过期连接
		for _, pc := range conns {
			if now.Sub(pc.createdAt) > p.maxLifetime {
				if p.close != nil {
					p.close(pc.conn)
				}
				continue
			}

			if now.Sub(pc.lastUsed) > 10*time.Minute {
				if p.close != nil {
					p.close(pc.conn)
				}
				continue
			}

			// 归还有效连接
			select {
			case p.conns <- pc:
			default:
				if p.close != nil {
					p.close(pc.conn)
				}
			}
		}
	}
}

// 错误定义
var (
	ErrPoolClosed = &PoolError{Code: "POOL_CLOSED", Message: "连接池已关闭"}
	ErrPoolFull   = &PoolError{Code: "POOL_FULL", Message: "连接池已满"}
)

// PoolError 连接池错误
type PoolError struct {
	Code    string
	Message string
}

func (e *PoolError) Error() string {
	return e.Code + ": " + e.Message
}
