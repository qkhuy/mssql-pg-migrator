package main

import (
	"context"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/qkhuy/mssql-pg-migrator/internal/app"
	"github.com/qkhuy/mssql-pg-migrator/internal/assess"
	"github.com/qkhuy/mssql-pg-migrator/internal/pipeline"

	// Registered engines (same set as the CLI).
	_ "github.com/qkhuy/mssql-pg-migrator/internal/source/demo"
	_ "github.com/qkhuy/mssql-pg-migrator/internal/source/mssql"
	_ "github.com/qkhuy/mssql-pg-migrator/internal/target/postgres"
)

// App is the Wails-bound object. Every exported method becomes callable from
// the Svelte frontend as window.go.main.App.<Method>. It delegates to the same
// internal/app.Service the CLI uses, so CLI and UI behave identically.
type App struct {
	ctx context.Context
	svc *app.Service
}

func NewApp() *App { return &App{svc: app.New()} }

// startup captures the Wails context (needed to emit events to the frontend).
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// Engines lists registered source/target engines for the connection screen.
func (a *App) Engines() map[string][]string {
	src, dst := a.svc.Engines()
	return map[string][]string{"sources": src, "targets": dst}
}

// Assess returns the full assessment (JSON-serialized to the frontend, which
// renders the table/column/type mappings and status badges).
func (a *App) Assess(src, dst app.Endpoint) (*assess.Assessment, error) {
	return a.svc.Assess(a.ctx, src, dst)
}

// AssessHTML returns the rendered HTML report (for an in-app preview/export).
func (a *App) AssessHTML(src, dst app.Endpoint) (string, error) {
	return a.svc.AssessHTML(a.ctx, src, dst)
}

// Plan performs a dry-run and returns the DDL + table plan.
func (a *App) Plan(src, dst app.Endpoint, opts app.RunOptions) (*pipeline.Report, error) {
	return a.svc.Plan(a.ctx, src, dst, opts)
}

// Run executes the migration, emitting a "progress" event per update so the UI
// can show live per-table progress bars. Returns the final report.
func (a *App) Run(src, dst app.Endpoint, opts app.RunOptions) (*pipeline.Report, error) {
	return a.svc.Run(a.ctx, src, dst, opts, func(p pipeline.Progress) {
		wruntime.EventsEmit(a.ctx, "progress", p)
	})
}
