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

	"github.com/qkhuy/mssql-pg-migrator/internal/config"
	"github.com/qkhuy/mssql-pg-migrator/internal/pipeline"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"

	// Registered engines. Add a line here when you add an adapter.
	_ "github.com/qkhuy/mssql-pg-migrator/internal/source/mssql"
	_ "github.com/qkhuy/mssql-pg-migrator/internal/target/postgres"
)

func main() {
	cfgPath := flag.String("config", "", "path to JSON config file")
	dryRun := flag.Bool("dry-run", false, "render DDL and plan without writing to the target")
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

	if err := run(*cfgPath, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfgPath string, dryRun bool) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	ctx := context.Background()

	src, err := source.Open(ctx, cfg.Source.Engine, cfg.Source.DSN)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := target.Open(ctx, cfg.Target.Engine, cfg.Target.DSN)
	if err != nil {
		return err
	}
	defer dst.Close()

	m := &pipeline.Migrator{
		Src: src,
		Dst: dst,
		Opts: pipeline.Options{
			DryRun:      dryRun || cfg.Migration.DryRun,
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
