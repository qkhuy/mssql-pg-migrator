// Package postgres is the PostgreSQL target adapter. It registers itself as the
// "postgres" target engine on import. The methods are stubbed; wiring the real
// driver (github.com/jackc/pgx/v5, using pgx.CopyFrom for bulk load) and the
// DDL generation is the next implementation step.
package postgres

import (
	"context"
	"errors"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"
)

func init() {
	target.Register("postgres", func() target.Target { return &Target{} })
}

// Target implements target.Target for PostgreSQL.
type Target struct {
	dsn string
	// pool *pgxpool.Pool
}

var errNotImplemented = errors.New("postgres target: not implemented yet")

func (t *Target) Open(ctx context.Context, dsn string) error {
	t.dsn = dsn
	// TODO: pgxpool.New(ctx, dsn); verify with Ping.
	return errNotImplemented
}

func (t *Target) Close() error { return nil }

// RenderDDL will map each ir.CanonicalType to a PostgreSQL type and emit
// CREATE TABLE / PRIMARY KEY statements, collecting a Warning for every lossy
// or unsupported construct so the user can review before applying.
func (t *Target) RenderDDL(s *ir.Schema) ([]string, []target.Warning, error) {
	return nil, nil, errNotImplemented
}

func (t *Target) ApplySchema(ctx context.Context, s *ir.Schema) error {
	return errNotImplemented
}

// BulkLoad will use the PostgreSQL COPY protocol via pgx.CopyFrom for maximum
// throughput on large tables.
func (t *Target) BulkLoad(ctx context.Context, table *ir.Table, rows <-chan ir.Row) (int64, error) {
	return 0, errNotImplemented
}
