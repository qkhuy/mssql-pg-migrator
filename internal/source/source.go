// Package source defines the read-only adapter contract for source databases
// and a registry so engines can be plugged in by name. Add a new source engine
// by implementing Source and calling Register from the adapter's init().
package source

import (
	"context"
	"fmt"
	"sort"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
)

// Source is a read-only adapter over a source database. Implementations MUST
// NOT mutate the source in any way.
type Source interface {
	// Open establishes a connection from a DSN.
	Open(ctx context.Context, dsn string) error
	Close() error

	// Introspect reads the full schema into the canonical IR.
	Introspect(ctx context.Context) (*ir.Schema, error)

	// Read returns a pull-based stream of a table's rows within the given
	// range. A zero Range (empty Column) reads the whole table. The error
	// return is for failures opening the stream; per-row failures surface via
	// the stream's Err().
	Read(ctx context.Context, table *ir.Table, r Range) (ir.RowStream, error)
}

// Counter is an optional capability: report a table's exact row count, used by
// the pipeline to validate a migration. Adapters that can't count are simply
// skipped during validation.
type Counter interface {
	Count(ctx context.Context, table *ir.Table) (int64, error)
}

// Range bounds a chunk by a single key column for parallel, resumable reads.
// Min is inclusive, Max is exclusive. A zero-value Range means "whole table".
type Range struct {
	Column string
	Min    any
	Max    any
}

// Factory constructs an unopened Source.
type Factory func() Source

var registry = map[string]Factory{}

// Register makes a source engine available under name. Call it from the
// adapter package's init(). Panics on duplicate registration.
func Register(name string, f Factory) {
	if _, dup := registry[name]; dup {
		panic("source: duplicate registration for engine " + name)
	}
	registry[name] = f
}

// Open looks up the named engine and opens a connection.
func Open(ctx context.Context, name, dsn string) (Source, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("source: unknown engine %q (available: %v)", name, Engines())
	}
	s := f()
	if err := s.Open(ctx, dsn); err != nil {
		return nil, fmt.Errorf("source %q: open: %w", name, err)
	}
	return s, nil
}

// Engines returns the registered source engine names, sorted.
func Engines() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
