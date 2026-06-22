// Command migrator is the CLI entry point. Source and target engines are
// plugged in via blank imports below: importing an adapter package runs its
// init(), which registers it. Supporting a new engine is one new package plus
// one line here — no changes to the pipeline.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

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

func main() {
	cfgPath := flag.String("config", "", "path to JSON config file")
	dryRun := flag.Bool("dry-run", false, "render DDL and plan without writing to the target")
	doAssess := flag.Bool("assess", false, "generate a read-only assessment report and exit")
	reportFormat := flag.String("format", "html", "assessment report format: html | md")
	reportOut := flag.String("out", "", "assessment report output file (default: stdout)")
	listEngines := flag.Bool("engines", false, "list registered source and target engines and exit")
	flag.Parse()

	if *listEngines {
		fmt.Printf("sources: %v\ntargets: %v\n", source.Engines(), target.Engines())
		return
	}

	if *cfgPath == "" {
		fmt.Fprintln(os.Stderr, "error: -config is required (or use -engines)")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fail(err)
	}

	if *doAssess {
		if err := runAssess(cfg, *reportFormat, *reportOut); err != nil {
			fail(err)
		}
		return
	}

	if err := run(cfg, *dryRun); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

// runAssess introspects the source and maps it through the target's pure
// mapping logic (no target connection needed), then writes the report.
func runAssess(cfg *config.Config, format, out string) error {
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
	switch format {
	case "md", "markdown":
		rendered, err = report.Markdown(a)
	case "html":
		rendered, err = report.HTML(a)
	default:
		return fmt.Errorf("unknown report format %q (use html or md)", format)
	}
	if err != nil {
		return err
	}

	if out == "" {
		fmt.Print(rendered)
		return nil
	}
	if err := os.WriteFile(out, []byte(rendered), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "assessment report written to %s\n", out)
	return nil
}

func run(cfg *config.Config, dryRun bool) error {
	ctx := context.Background()
	dryRun = dryRun || cfg.Migration.DryRun

	src, err := source.Open(ctx, cfg.Source.Engine, cfg.Source.DSN)
	if err != nil {
		return err
	}
	defer src.Close()

	// Dry-run renders DDL only and needs no live target connection.
	var dst target.Target
	if dryRun {
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
			DryRun:      dryRun,
			Parallelism: cfg.Migration.Parallelism,
			Tables:      cfg.Migration.Tables,
		},
	}

	report, err := m.Run(ctx)
	if err != nil {
		return err
	}

	printReport(report)
	return nil
}

func printReport(r *pipeline.Report) {
	if r.DryRun {
		fmt.Println("== DRY RUN ==")
	}
	if len(r.DDL) > 0 {
		fmt.Printf("-- %d DDL statement(s) --\n", len(r.DDL))
		for _, s := range r.DDL {
			fmt.Println(s)
		}
	}
	for _, w := range r.Warnings {
		fmt.Printf("WARN %s: %s\n", w.Object, w.Message)
	}
	for _, t := range r.Tables {
		status := "ok"
		if t.Err != nil {
			status = "error: " + t.Err.Error()
		}
		fmt.Printf("table %-30s read=%-10d written=%-10d %s\n", t.Table, t.RowsRead, t.RowsWritten, status)
	}
}
