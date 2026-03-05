package cftp

import (
	"context"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

var ErrPoolClosed = errors.New("ftp pool closed")

type Pool struct {
	mu                sync.Mutex
	closed            bool
	idleConns         chan *pooledConn
	sem               *semaphore.Weighted
	wg                sync.WaitGroup
	cfg               *PoolConfig
	done              chan struct{}
	cancelIdleCleanup context.CancelFunc
	factory           func(context.Context) (*ftp.ServerConn, int, error)
}

type PoolConfig struct {
	IdleTimeout        time.Duration `yaml:"idle_timeout"`
	MaxLifetime        time.Duration `yaml:"max_lifetime"`
	HealthCheckTimeout time.Duration `yaml:"health_check_timeout"`
	CleanupInterval    time.Duration `yaml:"cleanup_interval"`
	MaxIdle            int           `yaml:"max_idle"`
	MaxTotal           int           `yaml:"max_total"`
}

type pooledConn struct {
	conn       *ftp.ServerConn
	createdAt  time.Time
	lastUsedAt time.Time
}

func NewPool(ctx context.Context,
	cancelIdleCleanup context.CancelFunc,
	factory func(context.Context) (*ftp.ServerConn, int, error),
	cfg *PoolConfig) *Pool {

	if cfg.MaxTotal <= 0 {
		cfg.MaxTotal = 1
	}

	if cfg.MaxTotal < cfg.MaxIdle {
		cfg.MaxTotal = cfg.MaxIdle
	}

	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = time.Minute
	}

	if cfg.MaxLifetime <= 0 {
		cfg.MaxLifetime = 10 * time.Minute
	}

	pool := &Pool{
		idleConns:         make(chan *pooledConn, cfg.MaxIdle),
		sem:               semaphore.NewWeighted(int64(cfg.MaxTotal)),
		cfg:               cfg,
		done:              make(chan struct{}),
		cancelIdleCleanup: cancelIdleCleanup,
		factory:           factory,
	}

	go pool.cleanupIdle(ctx)

	return pool
}

func (p *Pool) cleanupIdle(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.CleanupInterval)

	defer close(p.done)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupIdleCons(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pool) cleanupIdleCons(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	var idleConns []*pooledConn

	drained := len(p.idleConns)

	for i := 0; i < drained; i++ {
		select {
		case c := <-p.idleConns:
			idleConns = append(idleConns, c)
		default:
			goto drainDone
		}
	}

drainDone:

	for _, c := range idleConns {
		if err := p.checkConn(ctx, c); err != nil {
			c.conn.Quit()
			continue
		}

		select {
		case p.idleConns <- c:
		default:
			c.conn.Quit()
		}
	}
}

func (p *Pool) checkConn(ctx context.Context, c *pooledConn) error {
	if time.Since(c.createdAt) > p.cfg.MaxLifetime {
		return errors.New("connection expired")
	}

	if time.Since(c.lastUsedAt) > p.cfg.IdleTimeout {
		return errors.New("connection idle timeout")
	}

	ictx, cancel := context.WithTimeout(ctx, p.cfg.HealthCheckTimeout)
	defer cancel()

	select {
	case <-ictx.Done():
		return errors.Wrap(ictx.Err(), "health check timeout")
	default:
		if err := c.conn.NoOp(); err != nil {
			return errors.Wrap(err, "health check failed")
		}
	}

	return nil
}

func (p *Pool) Close() error {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()

		return ErrPoolClosed
	}

	p.closed = true

	p.mu.Unlock()

	p.cancelIdleCleanup()

	<-p.done

	p.wg.Wait()

	p.mu.Lock()

	close(p.idleConns)

	for c := range p.idleConns {
		c.conn.Quit()
	}

	p.mu.Unlock()

	return nil
}

func (p *Pool) Get(ctx context.Context) (*ftp.ServerConn, int, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()

		return nil, 0, ErrPoolClosed
	}

	p.mu.Unlock()

	if err := p.sem.Acquire(ctx, 1); err != nil {
		return nil, 0, errors.Wrap(err, "acquire semaphore")
	}

	p.mu.Lock()

	var c *pooledConn

	select {
	case c = <-p.idleConns:
	default:
		c = nil
	}

	p.mu.Unlock()

	if c != nil {
		if err := p.checkConn(ctx, c); err != nil {
			c.conn.Quit()
			p.sem.Release(1)

			return p.createConn(ctx)
		}

		p.wg.Add(1)

		c.lastUsedAt = time.Now()

		return c.conn, 0, nil
	}

	return p.createConn(ctx)
}

func (p *Pool) createConn(ctx context.Context) (*ftp.ServerConn, int, error) {
	conn, stat, err := p.factory(ctx)
	if err != nil {
		p.sem.Release(1)

		return nil, stat, errors.Wrap(err, "create failed")
	}

	p.wg.Add(1)

	return conn, stat, nil
}

func (p *Pool) Put(conn *ftp.ServerConn) {
	defer p.wg.Done()

	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()

		conn.Quit()

		p.sem.Release(1)

		return
	}

	p.mu.Unlock()

	c := &pooledConn{
		conn:       conn,
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
	}

	p.mu.Lock()

	select {
	case p.idleConns <- c:
	default:
		conn.Quit()
	}

	p.mu.Unlock()

	p.sem.Release(1)
}
