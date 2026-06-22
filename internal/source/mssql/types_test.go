package mssql

import (
	"testing"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
)

func TestMapColumnType(t *testing.T) {
	cases := []struct {
		name       string
		typeName   string
		maxLen     int
		prec       int
		scale      int
		wantKind   ir.TypeKind
		wantNative string
	}{
		{"int", "int", 4, 0, 0, ir.KindInt, "int"},
		{"bigint", "bigint", 8, 0, 0, ir.KindInt, "bigint"},
		{"bit", "bit", 1, 0, 0, ir.KindBool, "bit"},
		{"decimal", "decimal", 0, 19, 4, ir.KindDecimal, "decimal(19,4)"},
		{"money", "money", 8, 0, 0, ir.KindDecimal, "money"},
		{"nvarchar", "nvarchar", 200, 0, 0, ir.KindString, "nvarchar(100)"},
		{"nvarchar(max)", "nvarchar", -1, 0, 0, ir.KindString, "nvarchar(max)"},
		{"varchar", "varchar", 50, 0, 0, ir.KindString, "varchar(50)"},
		{"char", "char", 10, 0, 0, ir.KindString, "char(10)"},
		{"datetimeoffset", "datetimeoffset", 10, 0, 0, ir.KindTimestampTZ, "datetimeoffset"},
		{"datetime2", "datetime2", 8, 0, 0, ir.KindTimestamp, "datetime2"},
		{"uniqueidentifier", "uniqueidentifier", 16, 0, 0, ir.KindUUID, "uniqueidentifier"},
		{"geography", "geography", -1, 0, 0, ir.KindUnknown, "geography"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapColumnType(c.typeName, c.maxLen, c.prec, c.scale)
			if got.Kind != c.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, c.wantKind)
			}
			if got.Native != c.wantNative {
				t.Errorf("Native = %q, want %q", got.Native, c.wantNative)
			}
		})
	}
}

func TestNvarcharLength(t *testing.T) {
	got := mapColumnType("nvarchar", 200, 0, 0)
	if got.Length != 100 {
		t.Errorf("nvarchar byte-length 200 should map to char length 100, got %d", got.Length)
	}
}
