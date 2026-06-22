package mssql

import (
	"fmt"
	"strings"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
)

// mapColumnType maps a SQL Server native type to a canonical type. maxLen is
// sys.columns.max_length (bytes; -1 means MAX). precision/scale apply to
// numeric types. The returned type's Native field holds a display string used
// by the assessment report.
func mapColumnType(typeName string, maxLen, precision, scale int) ir.CanonicalType {
	t := strings.ToLower(typeName)
	ct := ir.CanonicalType{}
	native := t

	switch t {
	case "bit":
		ct.Kind = ir.KindBool
	case "tinyint":
		ct.Kind, ct.BitWidth = ir.KindInt, 8
	case "smallint":
		ct.Kind, ct.BitWidth, ct.Signed = ir.KindInt, 16, true
	case "int":
		ct.Kind, ct.BitWidth, ct.Signed = ir.KindInt, 32, true
	case "bigint":
		ct.Kind, ct.BitWidth, ct.Signed = ir.KindInt, 64, true
	case "real":
		ct.Kind, ct.BitWidth = ir.KindFloat, 32
	case "float":
		ct.Kind, ct.BitWidth = ir.KindFloat, 64
	case "decimal", "numeric":
		ct.Kind, ct.Precision, ct.Scale = ir.KindDecimal, precision, scale
		native = fmt.Sprintf("%s(%d,%d)", t, precision, scale)
	case "money":
		ct.Kind, ct.Precision, ct.Scale = ir.KindDecimal, 19, 4
	case "smallmoney":
		ct.Kind, ct.Precision, ct.Scale = ir.KindDecimal, 10, 4
	case "char", "nchar":
		ct.Kind, ct.Fixed, ct.Length = ir.KindString, true, charLen(t, maxLen)
		native = fmt.Sprintf("%s(%d)", t, ct.Length)
	case "varchar", "nvarchar":
		ct.Kind = ir.KindString
		if maxLen == -1 {
			ct.Length, native = 0, t+"(max)"
		} else {
			ct.Length = charLen(t, maxLen)
			native = fmt.Sprintf("%s(%d)", t, ct.Length)
		}
	case "text", "ntext":
		ct.Kind = ir.KindText
	case "binary", "varbinary":
		ct.Kind = ir.KindBinary
		if maxLen == -1 {
			native = t + "(max)"
		}
	case "image":
		ct.Kind = ir.KindBinary
	case "date":
		ct.Kind = ir.KindDate
	case "time":
		ct.Kind = ir.KindTime
	case "datetime", "datetime2", "smalldatetime":
		ct.Kind = ir.KindTimestamp
	case "datetimeoffset":
		ct.Kind = ir.KindTimestampTZ
	case "uniqueidentifier":
		ct.Kind = ir.KindUUID
	default:
		// geography, geometry, hierarchyid, sql_variant, xml, ... — no clean
		// mapping; flagged for manual review by the target/report layer.
		ct.Kind = ir.KindUnknown
	}

	ct.Native = native
	return ct
}

// charLen converts a byte length to a character length (Unicode types store
// two bytes per character in sys.columns.max_length).
func charLen(t string, maxLen int) int {
	if maxLen < 0 {
		return 0
	}
	if strings.HasPrefix(t, "n") {
		return maxLen / 2
	}
	return maxLen
}

func routineKind(sqlType string) ir.RoutineKind {
	switch strings.TrimSpace(sqlType) {
	case "P":
		return ir.RoutineProcedure
	case "TR":
		return ir.RoutineTrigger
	default: // FN, IF, TF
		return ir.RoutineFunction
	}
}
