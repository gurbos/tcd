package datastore

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pgxp "github.com/jackc/pgx/v5/pgxpool"
)

// Config creates pgxpool.Config with defualt settings provided
// by the parameters.
func Config(dsn string) *pgxpool.Config {
	const defaultMaxConns = int32(4)
	const defaultMinConns = int32(2)
	const defaultMaxConnLifetime = time.Minute * 10
	const defaultMaxIdletime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	config, err := pgxp.ParseConfig(dsn)
	if err != nil {
		log.Fatal(fmt.Errorf("Error parsing dsn to config: %W", err))
	}

	config.MaxConns = defaultMaxConns
	config.MinConns = defaultMinConns
	config.MaxConnLifetime = defaultMaxConnLifetime
	config.MaxConnIdleTime = defaultMaxIdletime
	config.HealthCheckPeriod = defaultHealthCheckPeriod
	config.ConnConfig.ConnectTimeout = defaultConnectTimeout
	return config
}

// NewDBPool creates a new PostgreSQL connection pool using the provided connection string.
func NewDBPool(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
	cp, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("Error in NewDBPool: %w", err)
	}
	return cp, nil
}

// Initialize a new PostgresDataRepository with a connection pool.
func NewPostgresDataStore(pool *pgxpool.Pool) *PostgresDataStore {
	return &PostgresDataStore{cp: pool}
}
