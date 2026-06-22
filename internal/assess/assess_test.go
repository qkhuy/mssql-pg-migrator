package assess

import (
	"strings"
	"testing"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/target/postgres"
)

func TestBuild(t *testing.T) {
	s := &ir.Schema{
		Tables: []*ir.Table{{
			Schema: "dbo", Name: "Orders", EstimatedRows: 1000,
			Columns: []*ir.Column{
				{Name: "ID", Type: ir.CanonicalType{Kind: ir.KindInt, BitWidth: 32}},
				{Name: "Geo", Type: ir.CanonicalType{Kind: ir.KindUnknown, Native: "geography"}},
			},
		}},
		Views:    []*ir.View{{Schema: "dbo", Name: "v1"}},
		Routines: []*ir.Routine{{Schema: "dbo", Name: "p1", Kind: ir.RoutineProcedure}},
	}

	a := Build("mssql", "postgres", s, &postgres.Target{})

	if a.Summary.TotalTables != 1 || a.Summary.TotalColumns != 2 ||
		a.Summary.TotalViews != 1 || a.Summary.TotalRoutines != 1 {
		t.Fatalf("summary counts wrong: %+v", a.Summary)
	}
	// Objects tallied: 2 columns + 1 view + 1 routine = 4 (1 auto, 3 review/unsupported).
	if a.Summary.AutoObjects != 1 || a.Summary.UnsupportedObjects != 1 || a.Summary.ReviewObjects != 2 {
		t.Errorf("status tally wrong: %+v", a.Summary)
	}
	if a.Summary.AutoPercent != 25 { // 1/4
		t.Errorf("AutoPercent = %d, want 25", a.Summary.AutoPercent)
	}

	tm := a.Tables[0]
	if tm.Target != "dbo.orders" {
		t.Errorf("target table name = %q, want dbo.orders", tm.Target)
	}
	if tm.Status != StatusUnsupported {
		t.Errorf("table status = %v, want Unsupported (geography column)", tm.Status)
	}
	if got := tm.Columns[1]; got.TargetType != "text" || got.Status != StatusUnsupported {
		t.Errorf("geography column mapping wrong: %+v", got)
	}
}

func TestStatusLabel(t *testing.T) {
	if !strings.Contains(StatusAuto.Label(), "Tự động") {
		t.Errorf("unexpected label: %q", StatusAuto.Label())
	}
}
