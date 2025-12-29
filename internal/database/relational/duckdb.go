// Package relational provides DuckDB-backed persistence for system metrics.
package relational

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/marcboeker/go-duckdb" // Register DuckDB driver
)

// =============================================================================
// DATABASE CLIENT INTERFACE
// =============================================================================

// DatabaseClient defines the contract for database operations.
type DatabaseClient interface {
	// DB returns the underlying sql.DB instance.
	DB() *sql.DB
	// Close releases database resources.
	Close() error
	// Configure sets database-specific options.
	Configure(opts DatabaseConfig) error
	// Ping verifies database connectivity.
	Ping(ctx context.Context) error
}

// DatabaseConfig holds configuration options for the database.
type DatabaseConfig struct {
	Threads       int           // Number of threads for DuckDB (0 = default)
	MemoryLimitGB int           // Memory limit in GB (0 = default)
	Timeout       time.Duration // Query timeout (0 = no timeout)
}

// =============================================================================
// DUCKDB CLIENT IMPLEMENTATION
// =============================================================================

// DuckDBClient manages the physical connection to a DuckDB database.
type DuckDBClient struct {
	db     *sql.DB
	config DatabaseConfig
}

// DuckDBOption configures the DuckDB client.
type DuckDBOption func(*DuckDBClient)

// WithThreads sets the number of DuckDB threads.
func WithThreads(n int) DuckDBOption {
	return func(c *DuckDBClient) {
		c.config.Threads = n
	}
}

// WithMemoryLimit sets the DuckDB memory limit in GB.
func WithMemoryLimit(gb int) DuckDBOption {
	return func(c *DuckDBClient) {
		c.config.MemoryLimitGB = gb
	}
}

// WithTimeout sets the query timeout.
func WithTimeout(d time.Duration) DuckDBOption {
	return func(c *DuckDBClient) {
		c.config.Timeout = d
	}
}

// NewDuckDBClient creates a new DuckDB client.
// If dsn is empty, an in-memory database is created.
// DSN examples:
//   - "" or ":memory:" for in-memory database
//   - "/path/to/file.db" for file-based database
//   - "/path/to/file.db?access_mode=READ_WRITE" with options
func NewDuckDBClient(dsn string, opts ...DuckDBOption) (*DuckDBClient, error) {
	client := &DuckDBClient{
		config: DatabaseConfig{
			Threads:       0, // DuckDB default
			MemoryLimitGB: 0, // DuckDB default
			Timeout:       0, // No timeout
		},
	}

	// Apply options
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}

	// Normalize DSN
	if dsn == "" {
		dsn = ":memory:"
	}

	// Open database connection
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	// Verify connectivity
	ctx := context.Background()
	if client.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, client.config.Timeout)
		defer cancel()
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping duckdb: %w", err)
	}

	// DuckDB is embedded; serial access is often safer/faster for writes
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Connections don't expire

	client.db = db

	// Apply configuration
	if err := client.Configure(client.config); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to configure duckdb: %w", err)
	}

	return client, nil
}

// DB returns the underlying sql.DB instance.
func (c *DuckDBClient) DB() *sql.DB {
	return c.db
}

// Close releases database resources.
func (c *DuckDBClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Configure applies database configuration options.
func (c *DuckDBClient) Configure(cfg DatabaseConfig) error {
	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if cfg.Threads > 0 {
		_, err := c.db.Exec(fmt.Sprintf("PRAGMA threads=%d", cfg.Threads))
		if err != nil {
			return fmt.Errorf("setting threads: %w", err)
		}
	}

	if cfg.MemoryLimitGB > 0 {
		_, err := c.db.Exec(fmt.Sprintf("PRAGMA memory_limit='%dGB'", cfg.MemoryLimitGB))
		if err != nil {
			return fmt.Errorf("setting memory limit: %w", err)
		}
	}

	c.config = cfg
	return nil
}

// Ping verifies database connectivity.
func (c *DuckDBClient) Ping(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return c.db.PingContext(ctx)
}

// Exec executes a query that doesn't return rows.
func (c *DuckDBClient) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return c.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (c *DuckDBClient) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return c.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (c *DuckDBClient) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}

// BeginTx starts a new transaction.
func (c *DuckDBClient) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return c.db.BeginTx(ctx, opts)
}

// =============================================================================
// FACTORY FUNCTIONS
// =============================================================================

// NewInMemoryDB creates a new in-memory DuckDB database.
func NewInMemoryDB(opts ...DuckDBOption) (*DuckDBClient, error) {
	return NewDuckDBClient(":memory:", opts...)
}

// NewFileDB creates a new file-based DuckDB database.
func NewFileDB(path string, opts ...DuckDBOption) (*DuckDBClient, error) {
	if path == "" {
		return nil, fmt.Errorf("database path required")
	}
	return NewDuckDBClient(path, opts...)
}
