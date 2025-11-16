package httpexecutor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luist18/halo/internal/cache"
	"github.com/luist18/halo/internal/data"
)

const poolEntryTTL = 10 * time.Minute

var (
	poolCacheOnce sync.Once
	poolCache     *cache.TTLCache[string, *pgxpool.Pool]
)

type managedConn interface {
	queryExecutor
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Close(ctx context.Context) error
}

type directConnection struct {
	conn *pgx.Conn
}

func (d *directConnection) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return d.conn.Query(ctx, query, args...)
}

func (d *directConnection) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return d.conn.BeginTx(ctx, txOptions)
}

func (d *directConnection) Close(ctx context.Context) error {
	if d.conn == nil {
		return nil
	}
	return d.conn.Close(ctx)
}

type pooledConnection struct {
	conn *pgxpool.Conn
}

func (p *pooledConnection) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return p.conn.Query(ctx, query, args...)
}

func (p *pooledConnection) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return p.conn.BeginTx(ctx, txOptions)
}

func (p *pooledConnection) Close(ctx context.Context) error {
	if p.conn != nil {
		p.conn.Release()
	}
	return nil
}

func acquireConnection(ctx context.Context, connStrSecret data.Secret, poolOptIn bool) (managedConn, error) {
	if poolOptIn {
		pooledConn, err := acquirePooledConnection(ctx, connStrSecret)
		if err != nil {
			return nil, err
		}
		return pooledConn, nil
	}

	conn, err := createConnection(ctx, connStrSecret)
	if err != nil {
		return nil, err
	}

	return &directConnection{conn: conn}, nil
}

func acquirePooledConnection(ctx context.Context, connStrSecret data.Secret) (managedConn, error) {
	pool, err := getOrCreatePool(ctx, connStrSecret)
	if err != nil {
		return nil, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		removePoolFromCache(connStrSecret.Unwrap())
		return nil, fmt.Errorf("failed to acquire pooled connection: %w", err)
	}

	return &pooledConnection{conn: conn}, nil
}

func getOrCreatePool(ctx context.Context, connStrSecret data.Secret) (*pgxpool.Pool, error) {
	connStr := connStrSecret.Unwrap()

	pool, err := getPoolCache().GetOrCompute(connStr, func() (*pgxpool.Pool, error) {
		cfg, err := pgxpool.ParseConfig(connStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse connection string for pool: %w", err)
		}
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

		instance, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize connection pool: %w", err)
		}
		return instance, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return pool, nil
}

func getPoolCache() *cache.TTLCache[string, *pgxpool.Pool] {
	poolCacheOnce.Do(func() {
		poolCache = cache.NewTTLCache[string, *pgxpool.Pool](
			poolEntryTTL,
			cache.WithCleanupInterval[string, *pgxpool.Pool](time.Minute),
			cache.WithOnEvict[string, *pgxpool.Pool](func(_ string, pool *pgxpool.Pool) {
				if pool != nil {
					pool.Close()
				}
			}),
		)
	})
	return poolCache
}

func removePoolFromCache(key string) {
	getPoolCache().Delete(key)
}

func closeManagedConnection(ctx context.Context, conn managedConn) {
	if conn == nil {
		return
	}

	if err := conn.Close(ctx); err != nil {
		slog.Error("failed to close connection", slog.String("error", err.Error()))
	}
}
