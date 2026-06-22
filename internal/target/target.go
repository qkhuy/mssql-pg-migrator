// Package target defines the write adapter contract for destination databases
// and a registry so engines can be plugged in by name. Add a new target engine
// by implementing Target and calling Register from the adapter's init().
package target

import (
	"context"
	"fmt"
	"sort"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
)

// Target is a write adapter over a destination database.
type Target interface {
	Open(ctx context.Context, dsn string) error
	Close() error

	// RenderDDL produces dialect-specific DDL for the schema WITHOUT executing
	// it. This backs dry-run and the pre-migration review report. It returns
	// the statements plus warnings for every lossy or unsupported construct.
	RenderDDL(s *ir.Schema) (stmts []string, warnings []Warning, err error)

	// ApplySchema creates tables and constraints. Implementations should defer
	// secondary indexes and foreign keys until after bulk load (the pipeline
	// coordinates this) for speed.
	ApplySchema(ctx context.Context, s *ir.Schema) error

	// BulkLoad writes a stream of rows into a table using the fastest path the
	// engine offers (e.g. the COPY protocol for PostgreSQL). It returns the
	// number of rows written.
	BulkLoad(ctx context.Context, table *ir.Table, rows <-chan ir.Row) (int64, error)
}

// Warning flags a construct that could not be translated cleanly. It is shown
// in the report so a human can decide how to handle it — the tool never
// silently guesses around an unsupported construct.
type Warning struct {
	Object  string // e.g. "dbo.Orders.Total"
	Message string
}

// Factory constructs an unopened Target.
type Factory func() Target

var registry = map[string]Factory{}

// Register makes a target engine available under name. Call from init().
func Register(name string, f Factory) {
	if _, dup := registry[name]; dup {
		panic("target: duplicate registration for engine " + name)
	}
	registry[name] = f
}

// Open looks up the named engine and opens a connection.
func Open(ctx context.Context, name, dsn string) (Target, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("target: unknown engine %q (available: %v)", name, Engines())
	}
	t := f()
	if err := t.Open(ctx, dsn); err != nil {
		return nil, fmt.Errorf("target %q: open: %w", name, err)
	}
	return t, nil
}

// Engines returns the registered target engine names, sorted.
func Engines() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
