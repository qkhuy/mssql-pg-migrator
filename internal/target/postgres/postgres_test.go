package postgres

import (
	"testing"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
)

func TestMapType(t *testing.T) {
	tg := &Target{}
	cases := []struct {
		name  string
		ct    ir.CanonicalType
		want  string
		lossy bool
	}{
		{"bool", ir.CanonicalType{Kind: ir.KindBool}, "boolean", false},
		{"tinyint", ir.CanonicalType{Kind: ir.KindInt, BitWidth: 8}, "smallint", false},
		{"smallint", ir.CanonicalType{Kind: ir.KindInt, BitWidth: 16}, "smallint", false},
		{"int", ir.CanonicalType{Kind: ir.KindInt, BitWidth: 32}, "integer", false},
		{"bigint", ir.CanonicalType{Kind: ir.KindInt, BitWidth: 64}, "bigint", false},
		{"real", ir.CanonicalType{Kind: ir.KindFloat, BitWidth: 32}, "real", false},
		{"double", ir.CanonicalType{Kind: ir.KindFloat, BitWidth: 64}, "double precision", false},
		{"numeric", ir.CanonicalType{Kind: ir.KindDecimal, Precision: 19, Scale: 4}, "numeric(19,4)", false},
		{"varchar", ir.CanonicalType{Kind: ir.KindString, Length: 100}, "varchar(100)", false},
		{"char", ir.CanonicalType{Kind: ir.KindString, Length: 40, Fixed: true}, "char(40)", false},
		{"text", ir.CanonicalType{Kind: ir.KindString, Length: 0}, "text", false},
		{"bigtext", ir.CanonicalType{Kind: ir.KindText}, "text", false},
		{"binary", ir.CanonicalType{Kind: ir.KindBinary}, "bytea", false},
		{"uuid", ir.CanonicalType{Kind: ir.KindUUID}, "uuid", false},
		{"timestamptz", ir.CanonicalType{Kind: ir.KindTimestampTZ}, "timestamptz", false},
		{"json", ir.CanonicalType{Kind: ir.KindJSON}, "jsonb", false},
		{"unknown", ir.CanonicalType{Kind: ir.KindUnknown, Native: "geography"}, "text", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tg.MapType(c.ct)
			if got.Native != c.want {
				t.Errorf("Native = %q, want %q", got.Native, c.want)
			}
			if got.Lossy != c.lossy {
				t.Errorf("Lossy = %v, want %v", got.Lossy, c.lossy)
			}
		})
	}
}

func TestMapIdentifier(t *testing.T) {
	tg := &Target{}
	if got := tg.MapIdentifier("Orders"); got != "orders" {
		t.Errorf("MapIdentifier(Orders) = %q, want orders", got)
	}
}

func TestRenderDDL(t *testing.T) {
	tg := &Target{}
	s := &ir.Schema{Tables: []*ir.Table{{
		Schema: "dbo", Name: "Orders",
		Columns: []*ir.Column{
			{Name: "OrderID", Type: ir.CanonicalType{Kind: ir.KindInt, BitWidth: 64}, IsIdentity: true},
			{Name: "Total", Type: ir.CanonicalType{Kind: ir.KindDecimal, Precision: 19, Scale: 4}, Nullable: true},
			{Name: "Geo", Type: ir.CanonicalType{Kind: ir.KindUnknown, Native: "geography"}, Nullable: true},
		},
		PrimaryKey: &ir.PrimaryKey{Columns: []string{"OrderID"}},
	}}}
	stmts, warnings, err := tg.RenderDDL(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 {
		t.Fatalf("got %d statements, want 1", len(stmts))
	}
	want := "CREATE TABLE dbo.orders"
	if got := stmts[0]; len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("statement = %q, want prefix %q", got, want)
	}
	if len(warnings) != 1 {
		t.Errorf("got %d warnings, want 1 (the geography column)", len(warnings))
	}
}
