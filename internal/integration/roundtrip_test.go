//go:build integration

// Package integration holds a live end-to-end round trip, gated behind the
// `integration` build tag and skipped unless both DSNs are provided:
//
//	MIGRATOR_MSSQL_DSN=... MIGRATOR_PG_DSN=... go test -tags integration ./internal/integration/
package integration

import (
	"context"
	"os"
	"testing"

	"github.com/qkhuy/mssql-pg-migrator/internal/pipeline"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"

	_ "github.com/qkhuy/mssql-pg-migrator/internal/source/mssql"
	_ "github.com/qkhuy/mssql-pg-migrator/internal/target/postgres"
)

func TestRoundTrip(t *testing.T) {
	srcDSN := os.Getenv("MIGRATOR_MSSQL_DSN")
	dstDSN := os.Getenv("MIGRATOR_PG_DSN")
	if srcDSN == "" || dstDSN == "" {
		t.Skip("set MIGRATOR_MSSQL_DSN and MIGRATOR_PG_DSN to run")
	}

	ctx := context.Background()

	src, err := source.Open(ctx, "mssql", srcDSN)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer src.Close()

	dst, err := target.Open(ctx, "postgres", dstDSN)
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	defer dst.Close()

	m := &pipeline.Migrator{
		Src:  src,
		Dst:  dst,
		Opts: pipeline.Options{Parallelism: 4},
	}

	rep, err := m.Run(ctx)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !rep.Finalized {
		t.Error("expected finalize to run")
	}
	if !rep.Validated {
		t.Error("expected validation to run")
	}
	for _, tr := range rep.Tables {
		if tr.Err != nil {
			t.Errorf("table %s: %v", tr.Table, tr.Err)
		}
	}
}
