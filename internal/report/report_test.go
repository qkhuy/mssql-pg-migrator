package report

import (
	"strings"
	"testing"
	"time"

	"github.com/qkhuy/mssql-pg-migrator/internal/assess"
)

func TestComma(t *testing.T) {
	cases := map[int64]string{0: "0", 999: "999", 1000: "1,000", 11800000: "11,800,000", -1234: "-1,234"}
	for in, want := range cases {
		if got := comma(in); got != want {
			t.Errorf("comma(%d) = %q, want %q", in, got, want)
		}
	}
}

func sampleAssessment() *assess.Assessment {
	return &assess.Assessment{
		SourceEngine: "mssql", TargetEngine: "postgres", GeneratedAt: time.Now(),
		Schemas: []assess.SchemaMapping{{Source: "dbo", Target: "dbo"}},
		Summary: assess.Summary{TotalTables: 1, TotalColumns: 2, TotalRows: 1000, AutoPercent: 50, AutoObjects: 1, ReviewObjects: 1},
		Tables: []assess.TableMapping{{
			Source: "dbo.Orders", Target: "dbo.orders", EstimatedRows: 1000, Status: assess.StatusAuto,
			Columns: []assess.ColumnMapping{{Source: "ID", Target: "id", SourceType: "int", TargetType: "integer", Status: assess.StatusAuto}},
		}},
	}
}

func TestMarkdownRenders(t *testing.T) {
	out, err := Markdown(sampleAssessment())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Báo cáo", "dbo.Orders", "dbo.orders", "integer", "1,000"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestHTMLRenders(t *testing.T) {
	out, err := HTML(sampleAssessment())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!DOCTYPE html>", "badge", "dbo.orders", "<code>integer</code>"} {
		if !strings.Contains(out, want) {
			t.Errorf("html missing %q", want)
		}
	}
}
