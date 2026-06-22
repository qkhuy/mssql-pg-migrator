// Command migrator is the CLI entry point. Source and target engines are
// plugged in via blank imports below: importing an adapter package runs its
// init(), which registers it. Supporting a new engine is one new package plus
// one import line — no changes to the pipeline.
//
// Usage:
//
//	migrator <command> [flags]
//
// Commands: assess, plan, run, engines, version, help.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/qkhuy/mssql-pg-migrator/internal/assess"
	"github.com/qkhuy/mssql-pg-migrator/internal/config"
	"github.com/qkhuy/mssql-pg-migrator/internal/pipeline"
	"github.com/qkhuy/mssql-pg-migrator/internal/report"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"

	// Registered engines. Add a line here when you add an adapter.
	_ "github.com/qkhuy/mssql-pg-migrator/internal/source/demo"
	_ "github.com/qkhuy/mssql-pg-migrator/internal/source/mssql"
	_ "github.com/qkhuy/mssql-pg-migrator/internal/target/postgres"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	var err error
	switch cmd {
	case "engines":
		fmt.Printf("sources: %v\ntargets: %v\n", source.Engines(), target.Engines())
		return
	case "version", "-version", "--version", "-v":
		fmt.Printf("migrator %s\n", version)
		return
	case "help", "-h", "--help":
		usage()
		return
	case "assess":
		err = cmdAssess(args)
	case "plan":
		err = cmdRun(args, true)
	case "run":
		err = cmdRun(args, false)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `migrator %s — database migration tool

Usage:
  migrator <command> [flags]

Commands:
  assess    Read-only: render a visual report (HTML/Markdown) of the full plan
  plan      Dry-run: show the DDL and table plan without writing to the target
  run       Execute the migration (schema -> data -> finalize -> validate)
  engines   List registered source and target engines
  version   Print the version
  help      Show this help

Run "migrator <command> -h" for command flags.
`, version)
}

func cmdAssess(args []string) error {
	fs := flag.NewFlagSet("assess", flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to JSON config file (required)")
	format := fs.String("format", "html", "report format: html | md")
	out := fs.String("out", "", "output file (default: stdout)")
	_ = fs.Parse(args)

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}

	ctx := context.Background()
	src, err := source.Open(ctx, cfg.Source.Engine, cfg.Source.DSN)
	if err != nil {
		return err
	}
	defer src.Close()

	schema, err := src.Introspect(ctx)
	if err != nil {
		return fmt.Errorf("introspect: %w", err)
	}
	mapper, err := target.NewMapper(cfg.Target.Engine)
	if err != nil {
		return err
	}

	a := assess.Build(cfg.Source.Engine, cfg.Target.Engine, schema, mapper)

	var rendered string
	switch *format {
	case "md", "markdown":
		rendered, err = report.Markdown(a)
	case "html":
		rendered, err = report.HTML(a)
	default:
		return fmt.Errorf("unknown report format %q (use html or md)", *format)
	}
	if err != nil {
		return err
	}

	if *out == "" {
		fmt.Print(rendered)
		return nil
	}
	if err := os.WriteFile(*out, []byte(rendered), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "assessment report written to %s\n", *out)
	return nil
}

func cmdRun(args []string, dryRun bool) error {
	name := "run"
	if dryRun {
		name = "plan"
	}
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to JSON config file (required)")
	state := fs.String("state", ".migrator-state.json", "checkpoint file for resumability")
	parallelism := fs.Int("parallelism", 0, "tables migrated concurrently (0 = use config or default 4)")
	_ = fs.Parse(args)

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}

	dry := dryRun || cfg.Migration.DryRun
	par := *parallelism
	if par == 0 {
		par = cfg.Migration.Parallelism
	}

	ctx := context.Background()
	src, err := source.Open(ctx, cfg.Source.Engine, cfg.Source.DSN)
	if err != nil {
		return err
	}
	defer src.Close()

	// Dry-run renders DDL only and needs no live target connection.
	var dst target.Target
	if dry {
		dst, err = target.New(cfg.Target.Engine)
	} else {
		dst, err = target.Open(ctx, cfg.Target.Engine, cfg.Target.DSN)
	}
	if err != nil {
		return err
	}
	defer dst.Close()

	m := &pipeline.Migrator{
		Src: src,
		Dst: dst,
		Opts: pipeline.Options{
			DryRun:      dry,
			Parallelism: par,
			Tables:      cfg.Migration.Tables,
			StateFile:   *state,
		},
	}

	rep, runErr := m.Run(ctx)
	if rep != nil {
		printReport(rep)
	}
	return runErr
}

func loadConfig(path string) (*config.Config, error) {
	if path == "" {
		return nil, fmt.Errorf("-config is required")
	}
	return config.Load(path)
}

func printReport(r *pipeline.Report) {
	if r.DryRun {
		fmt.Println("== DRY RUN (no changes written) ==")
	}
	if len(r.DDL) > 0 {
		fmt.Printf("\n-- %d DDL statement(s) --\n", len(r.DDL))
		for _, s := range r.DDL {
			fmt.Println(s)
		}
	}
	if len(r.Warnings) > 0 {
		fmt.Printf("\n-- %d warning(s) (review needed) --\n", len(r.Warnings))
		for _, w := range r.Warnings {
			fmt.Printf("  ⚠ %s: %s\n", w.Object, w.Message)
		}
	}
	if len(r.Tables) > 0 {
		fmt.Printf("\n-- tables --\n")
		for _, t := range r.Tables {
			switch {
			case t.Skipped:
				fmt.Printf("  %-34s skipped (already done)\n", t.Table)
			case t.Err != nil:
				fmt.Printf("  %-34s ERROR: %v\n", t.Table, t.Err)
			case r.DryRun:
				fmt.Printf("  %-34s plan (~%d rows)\n", t.Table, t.SourceRows)
			default:
				fmt.Printf("  %-34s %d rows in %s\n", t.Table, t.RowsWritten, t.Duration.Round(time.Millisecond))
			}
		}
	}
	if !r.DryRun {
		fmt.Printf("\nfinalized=%v validated=%v\n", r.Finalized, r.Validated)
	}
}
