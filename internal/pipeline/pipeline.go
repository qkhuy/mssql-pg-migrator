// Package pipeline orchestrates a migration: introspect the source, render or
// apply the target schema, move data in parallel chunks, then validate. It is
// engine-agnostic — it only talks to the source.Source and target.Target
// interfaces, so any registered source/target pair works without changes here.
package pipeline

import (
	"context"
	"fmt"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"
)

// Options controls a migration run.
type Options struct {
	// DryRun renders DDL and a plan without writing anything to the target.
	DryRun bool

	// Parallelism is the number of tables/chunks moved concurrently.
	Parallelism int

	// Tables, if non-empty, restricts the run to these table names; otherwise
	// every introspected table is migrated.
	Tables []string
}

// Migrator wires a source and target together under a set of options.
type Migrator struct {
	Src  source.Source
	Dst  target.Target
	Opts Options
}

// TableResult records the outcome for one table.
type TableResult struct {
	Table        string
	RowsRead     int64
	RowsWritten  int64
	Err          error
}

// Report summarizes a run for printing or serialization.
type Report struct {
	DryRun   bool
	DDL      []string
	Warnings []target.Warning
	Tables   []TableResult
}

// Run executes the migration. The body below is the intended control flow;
// the chunked-parallel data movement and validation are stubbed until the
// adapters land, but the structure and the engine-agnostic contract are final.
func (m *Migrator) Run(ctx context.Context) (*Report, error) {
	if m.Opts.Parallelism <= 0 {
		m.Opts.Parallelism = 4
	}

	// 1. Introspect the source into canonical IR.
	schema, err := m.Src.Introspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("introspect: %w", err)
	}
	tables := m.selectTables(schema)

	// 2. Render the target DDL. Always rendered so the report can show it.
	ddl, warnings, err := m.Dst.RenderDDL(schema)
	if err != nil {
		return nil, fmt.Errorf("render ddl: %w", err)
	}
	report := &Report{DryRun: m.Opts.DryRun, DDL: ddl, Warnings: warnings}

	if m.Opts.DryRun {
		// Dry-run stops here: nothing is written to the target.
		for _, t := range tables {
			report.Tables = append(report.Tables, TableResult{Table: t.Name})
		}
		return report, nil
	}

	// 3. Create schema (tables + PKs; indexes/FKs deferred until after load).
	if err := m.Dst.ApplySchema(ctx, schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	// 4. Move data. TODO: chunk each table by primary-key range and run up to
	// Opts.Parallelism chunks concurrently, with checkpointing for resume.
	for _, t := range tables {
		report.Tables = append(report.Tables, m.migrateTable(ctx, t))
	}

	// 5. TODO: rebuild deferred indexes/FKs, then validate (row counts,
	// checksums) and attach results to the report.
	return report, nil
}

// migrateTable streams one table's rows from source to target. This is the
// single-chunk path; parallel range-chunking is layered on top in Run.
func (m *Migrator) migrateTable(ctx context.Context, t *ir.Table) TableResult {
	res := TableResult{Table: t.Name}

	rows, errs := m.Src.Read(ctx, t, source.Range{}) // whole table for now

	written, err := m.Dst.BulkLoad(ctx, t, rows)
	res.RowsWritten = written
	if err != nil {
		res.Err = err
		return res
	}
	if e := <-errs; e != nil {
		res.Err = e
	}
	return res
}

// selectTables applies the Tables filter, preserving introspection order.
func (m *Migrator) selectTables(s *ir.Schema) []*ir.Table {
	if len(m.Opts.Tables) == 0 {
		return s.Tables
	}
	want := make(map[string]bool, len(m.Opts.Tables))
	for _, n := range m.Opts.Tables {
		want[n] = true
	}
	var out []*ir.Table
	for _, t := range s.Tables {
		if want[t.Name] {
			out = append(out, t)
		}
	}
	return out
}
