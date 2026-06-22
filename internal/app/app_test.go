package app

import (
	"context"
	"testing"

	// Register the engines used by the tests.
	_ "github.com/qkhuy/mssql-pg-migrator/internal/source/demo"
	_ "github.com/qkhuy/mssql-pg-migrator/internal/target/postgres"
)

func TestEngines(t *testing.T) {
	src, dst := New().Engines()
	if !contains(src, "demo") || !contains(src, "mssql") {
		// mssql is registered by the CLI/UI, not here; only demo is guaranteed.
	}
	if !contains(src, "demo") {
		t.Errorf("sources missing demo: %v", src)
	}
	if !contains(dst, "postgres") {
		t.Errorf("targets missing postgres: %v", dst)
	}
}

func TestTestSourceDemo(t *testing.T) {
	if err := New().TestSource(context.Background(), Endpoint{Engine: "demo"}); err != nil {
		t.Errorf("TestSource(demo) = %v, want nil", err)
	}
}

func TestAssessDemoToPostgres(t *testing.T) {
	a, err := New().Assess(context.Background(),
		Endpoint{Engine: "demo"}, Endpoint{Engine: "postgres"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Summary.TotalTables != 3 {
		t.Errorf("TotalTables = %d, want 3", a.Summary.TotalTables)
	}
	if a.SourceEngine != "demo" || a.TargetEngine != "postgres" {
		t.Errorf("engines = %s->%s", a.SourceEngine, a.TargetEngine)
	}
	if a.Summary.AutoPercent <= 0 || a.Summary.AutoPercent > 100 {
		t.Errorf("AutoPercent out of range: %d", a.Summary.AutoPercent)
	}
}

func TestAssessUnknownTarget(t *testing.T) {
	_, err := New().Assess(context.Background(),
		Endpoint{Engine: "demo"}, Endpoint{Engine: "nope"})
	if err == nil {
		t.Error("expected error for unknown target engine")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
