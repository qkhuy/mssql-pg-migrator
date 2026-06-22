// Package app is a UI-agnostic service layer over the migration core. Both the
// CLI and the Wails desktop UI call this same service, so behavior stays
// identical across surfaces. It owns connection opening, introspection, and the
// pipeline wiring; callers supply only connection details, options, and an
// optional progress callback.
package app

import (
	"context"

	"github.com/qkhuy/mssql-pg-migrator/internal/assess"
	"github.com/qkhuy/mssql-pg-migrator/internal/pipeline"
	"github.com/qkhuy/mssql-pg-migrator/internal/report"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"
)

// Endpoint identifies a database engine and how to connect.
type Endpoint struct {
	Engine string `json:"engine"`
	DSN    string `json:"dsn"`
}

// RunOptions controls a migrate/plan run.
type RunOptions struct {
	Parallelism int      `json:"parallelism"`
	Tables      []string `json:"tables"`
	StateFile   string   `json:"stateFile"`
}

// Service exposes the migration operations. It is stateless and safe to reuse.
type Service struct{}

// New returns a Service.
func New() *Service { return &Service{} }

// Engines lists registered source and target engine names.
func (s *Service) Engines() (sources, targets []string) {
	return source.Engines(), target.Engines()
}

// Assess opens the source read-only, introspects it, and builds the assessment
// using the target's pure mapping logic (no target connection needed).
func (s *Service) Assess(ctx context.Context, src, dst Endpoint) (*assess.Assessment, error) {
	source0, err := source.Open(ctx, src.Engine, src.DSN)
	if err != nil {
		return nil, err
	}
	defer source0.Close()

	schema, err := source0.Introspect(ctx)
	if err != nil {
		return nil, err
	}
	mapper, err := target.NewMapper(dst.Engine)
	if err != nil {
		return nil, err
	}
	return assess.Build(src.Engine, dst.Engine, schema, mapper), nil
}

// AssessHTML / AssessMarkdown are convenience wrappers returning a rendered
// report — handy for the CLI's -out file and for embedding in the UI.
func (s *Service) AssessHTML(ctx context.Context, src, dst Endpoint) (string, error) {
	a, err := s.Assess(ctx, src, dst)
	if err != nil {
		return "", err
	}
	return report.HTML(a)
}

func (s *Service) AssessMarkdown(ctx context.Context, src, dst Endpoint) (string, error) {
	a, err := s.Assess(ctx, src, dst)
	if err != nil {
		return "", err
	}
	return report.Markdown(a)
}

// Plan performs a dry-run: it renders the DDL and table plan without writing to
// the target (no target connection required).
func (s *Service) Plan(ctx context.Context, src, dst Endpoint, opts RunOptions) (*pipeline.Report, error) {
	return s.migrate(ctx, src, dst, opts, true, nil)
}

// Run executes the migration end to end (schema → data → finalize → validate),
// streaming progress events to onProgress (may be nil).
func (s *Service) Run(ctx context.Context, src, dst Endpoint, opts RunOptions, onProgress pipeline.ProgressFunc) (*pipeline.Report, error) {
	return s.migrate(ctx, src, dst, opts, false, onProgress)
}

func (s *Service) migrate(ctx context.Context, src, dst Endpoint, opts RunOptions, dryRun bool, onProgress pipeline.ProgressFunc) (*pipeline.Report, error) {
	source0, err := source.Open(ctx, src.Engine, src.DSN)
	if err != nil {
		return nil, err
	}
	defer source0.Close()

	// A dry run renders DDL only and needs no live target connection.
	var dest target.Target
	if dryRun {
		dest, err = target.New(dst.Engine)
	} else {
		dest, err = target.Open(ctx, dst.Engine, dst.DSN)
	}
	if err != nil {
		return nil, err
	}
	defer dest.Close()

	m := &pipeline.Migrator{
		Src: source0,
		Dst: dest,
		Opts: pipeline.Options{
			DryRun:      dryRun,
			Parallelism: opts.Parallelism,
			Tables:      opts.Tables,
			StateFile:   opts.StateFile,
		},
		OnProgress: onProgress,
	}
	return m.Run(ctx)
}
