package main

import (
	"context"
	"os"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/qkhuy/mssql-pg-migrator/internal/app"
	"github.com/qkhuy/mssql-pg-migrator/internal/assess"
	"github.com/qkhuy/mssql-pg-migrator/internal/config"
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

// TestSource / TestTarget verify a connection; the frontend shows a green/red
// indicator. They resolve (no error) on success and reject on failure.
func (a *App) TestSource(e app.Endpoint) error { return a.svc.TestSource(a.ctx, e) }
func (a *App) TestTarget(e app.Endpoint) error { return a.svc.TestTarget(a.ctx, e) }

// SaveConfig builds a config from the current form values and writes it as YAML
// via a native save dialog. Returns the saved path, or "" if cancelled.
func (a *App) SaveConfig(src, dst app.Endpoint, opts app.RunOptions) (string, error) {
	c := &config.Config{
		Source:    config.Endpoint{Engine: src.Engine, DSN: src.DSN},
		Target:    config.Endpoint{Engine: dst.Engine, DSN: dst.DSN},
		Migration: config.Migration{Parallelism: opts.Parallelism, Tables: opts.Tables},
	}
	b, err := config.Marshal(c)
	if err != nil {
		return "", err
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           "Lưu config",
		DefaultFilename: "config.yaml",
		Filters:         []wruntime.FileFilter{{DisplayName: "YAML", Pattern: "*.yaml;*.yml"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadConfig opens a YAML/JSON config via a native dialog and returns it so the
// frontend can populate the form. Returns nil if the user cancelled.
func (a *App) LoadConfig() (*config.Config, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   "Nạp config",
		Filters: []wruntime.FileFilter{{DisplayName: "Config", Pattern: "*.yaml;*.yml;*.json"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return config.Parse(b)
}

// Assess returns the full assessment (JSON-serialized to the frontend, which
// renders the table/column/type mappings and status badges).
func (a *App) Assess(src, dst app.Endpoint) (*assess.Assessment, error) {
	return a.svc.Assess(a.ctx, src, dst)
}

// ExportAssessHTML renders the assessment to HTML and prompts a native save
// dialog. Returns the saved path, or "" if the user cancelled.
func (a *App) ExportAssessHTML(src, dst app.Endpoint) (string, error) {
	html, err := a.svc.AssessHTML(a.ctx, src, dst)
	if err != nil {
		return "", err
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           "Lưu báo cáo đánh giá",
		DefaultFilename: "assessment.html",
		Filters:         []wruntime.FileFilter{{DisplayName: "HTML", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return "", err
	}
	return path, nil
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
