// Package mssql is the SQL Server source adapter. It registers itself as the
// "mssql" source engine on import. The methods are stubbed; wiring the real
// driver (github.com/microsoft/go-mssqldb) and the type mapping is the next
// implementation step.
package mssql

import (
	"context"
	"errors"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
)

func init() {
	source.Register("mssql", func() source.Source { return &Source{} })
}

// Source implements source.Source for Microsoft SQL Server.
type Source struct {
	dsn string
	// db *sql.DB // opened with github.com/microsoft/go-mssqldb
}

var errNotImplemented = errors.New("mssql source: not implemented yet")

func (s *Source) Open(ctx context.Context, dsn string) error {
	s.dsn = dsn
	// TODO: sql.Open("sqlserver", dsn); verify with PingContext.
	return errNotImplemented
}

func (s *Source) Close() error { return nil }

// Introspect will read sys.tables / sys.columns / sys.indexes / FK catalog
// views and map native SQL Server types into ir.CanonicalType.
func (s *Source) Introspect(ctx context.Context) (*ir.Schema, error) {
	return nil, errNotImplemented
}

// Read will stream rows for the given primary-key range, ordered by PK, using
// a server-side cursor so memory stays bounded regardless of table size.
func (s *Source) Read(ctx context.Context, table *ir.Table, r source.Range) (<-chan ir.Row, <-chan error) {
	rows := make(chan ir.Row)
	errs := make(chan error, 1)
	go func() {
		defer close(rows)
		defer close(errs)
		errs <- errNotImplemented
	}()
	return rows, errs
}
