// Package pipeline orchestrates a migration: introspect the source, render or
// apply the target schema, move data in parallel (one table per worker, each
// streamed via the target's bulk-load path), finalize indexes/FKs, then
// validate. It is engine-agnostic — it only uses the source.Source and
// target.Target interfaces, so any registered source/target pair works here.
package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"
)

// Options controls a migration run.
type Options struct {
	DryRun      bool     // render DDL and plan without writing to the target
	Parallelism int      // number of tables migrated concurrently (default 4)
	Tables      []string // restrict to these table names (empty = all)
	StateFile   string   // checkpoint path for resumability (empty = disabled)
}

// Migrator wires a source and target together under a set of options.
type Migrator struct {
	Src  source.Source
	Dst  target.Target
	Opts Options
}

// TableResult records the outcome for one table.
type TableResult struct {
	Table       string
	RowsWritten int64
	SourceRows  int64 // from validation; -1 if not validated
	Duration    time.Duration
	Skipped     bool // already done per checkpoint
	Err         error
}

// Report summarizes a run.
type Report struct {
	DryRun    bool
	DDL       []string
	Warnings  []target.Warning
	Tables    []TableResult
	Finalized bool
	Validated bool
}

// Run executes the migration.
func (m *Migrator) Run(ctx context.Context) (*Report, error) {
	if m.Opts.Parallelism <= 0 {
		m.Opts.Parallelism = 4
	}

	schema, err := m.Src.Introspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("introspect: %w", err)
	}
	tables := m.selectTables(schema)

	ddl, warnings, err := m.Dst.RenderDDL(schema)
	if err != nil {
		return nil, fmt.Errorf("render ddl: %w", err)
	}
	report := &Report{DryRun: m.Opts.DryRun, DDL: ddl, Warnings: warnings}

	if m.Opts.DryRun {
		for _, t := range tables {
			report.Tables = append(report.Tables, TableResult{Table: qname(t), SourceRows: t.EstimatedRows})
		}
		return report, nil
	}

	cp, err := loadCheckpoint(m.Opts.StateFile)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: %w", err)
	}

	if !cp.SchemaApplied {
		if err := m.Dst.ApplySchema(ctx, schema); err != nil {
			return nil, fmt.Errorf("apply schema: %w", err)
		}
		cp.markSchemaApplied()
	}

	results := m.loadAll(ctx, tables, cp)
	report.Tables = results
	if failed(results) {
		return report, fmt.Errorf("%d table(s) failed; rerun to resume", countFailed(results))
	}

	if err := m.Dst.Finalize(ctx, schema); err != nil {
		return report, fmt.Errorf("finalize (indexes/foreign keys/sequences): %w", err)
	}
	report.Finalized = true

	m.validate(ctx, tables, report)
	cp.clear()
	return report, nil
}

// loadAll moves every not-yet-done table, up to Parallelism at a time. Indexes
// and foreign keys are created later (Finalize), so load order is irrelevant
// and tables can run concurrently.
func (m *Migrator) loadAll(ctx context.Context, tables []*ir.Table, cp *checkpoint) []TableResult {
	results := make([]TableResult, len(tables))
	sem := make(chan struct{}, m.Opts.Parallelism)
	var wg sync.WaitGroup

	for i, t := range tables {
		if cp.isDone(qname(t)) {
			results[i] = TableResult{Table: qname(t), Skipped: true, SourceRows: -1}
			continue
		}
		wg.Add(1)
		go func(i int, t *ir.Table) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := m.migrateTable(ctx, t)
			results[i] = res
			if res.Err == nil {
				cp.markDone(qname(t))
			}
		}(i, t)
	}
	wg.Wait()
	return results
}

func (m *Migrator) migrateTable(ctx context.Context, t *ir.Table) TableResult {
	start := time.Now()
	res := TableResult{Table: qname(t), SourceRows: -1}

	stream, err := m.Src.Read(ctx, t, source.Range{})
	if err != nil {
		res.Err = fmt.Errorf("read: %w", err)
		res.Duration = time.Since(start)
		return res
	}
	defer stream.Close()

	n, err := m.Dst.BulkLoad(ctx, t, stream)
	res.RowsWritten = n
	if err != nil {
		res.Err = fmt.Errorf("load: %w", err)
	} else if e := stream.Err(); e != nil {
		res.Err = fmt.Errorf("read: %w", e)
	}
	res.Duration = time.Since(start)
	return res
}

// validate compares source vs target row counts when both adapters support it.
func (m *Migrator) validate(ctx context.Context, tables []*ir.Table, report *Report) {
	srcCounter, okS := m.Src.(source.Counter)
	dstCounter, okT := m.Dst.(target.Counter)
	if !okS || !okT {
		return
	}
	report.Validated = true
	byName := map[string]int{}
	for i := range report.Tables {
		byName[report.Tables[i].Table] = i
	}
	for _, t := range tables {
		idx, ok := byName[qname(t)]
		if !ok {
			continue
		}
		sc, err := srcCounter.Count(ctx, t)
		if err != nil {
			continue
		}
		report.Tables[idx].SourceRows = sc
		dc, err := dstCounter.Count(ctx, t)
		if err != nil {
			continue
		}
		if sc != dc && report.Tables[idx].Err == nil {
			report.Tables[idx].Err = fmt.Errorf("row count mismatch: source=%d target=%d", sc, dc)
		}
	}
}

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
		if want[t.Name] || want[qname(t)] {
			out = append(out, t)
		}
	}
	return out
}

func qname(t *ir.Table) string {
	if t.Schema == "" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}

func failed(rs []TableResult) bool { return countFailed(rs) > 0 }

func countFailed(rs []TableResult) int {
	n := 0
	for _, r := range rs {
		if r.Err != nil {
			n++
		}
	}
	return n
}
