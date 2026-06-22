// Package ir defines the canonical, database-agnostic representation of a
// schema and its data. Source adapters translate a native database into this
// representation; target adapters translate it back out. Keeping a single
// canonical layer in the middle is what lets the tool support N source engines
// and M target engines with N+M adapters instead of N*M direct converters.
package ir

// TypeKind is the database-agnostic category of a column type. A source adapter
// maps each native type into a TypeKind (plus the attributes below); a target
// adapter maps a TypeKind back into its own native DDL.
type TypeKind int

const (
	// KindUnknown means the source type had no canonical mapping. The original
	// type is preserved in CanonicalType.Native and must be surfaced to the
	// user for manual review — never silently guessed.
	KindUnknown TypeKind = iota
	KindBool
	KindInt
	KindFloat
	KindDecimal
	KindString // bounded character data (CHAR/VARCHAR/NVARCHAR)
	KindText   // unbounded character data (TEXT/CLOB)
	KindBinary // BINARY/VARBINARY/BLOB
	KindDate
	KindTime
	KindTimestamp   // no timezone
	KindTimestampTZ // with timezone
	KindUUID
	KindJSON
)

func (k TypeKind) String() string {
	switch k {
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindDecimal:
		return "decimal"
	case KindString:
		return "string"
	case KindText:
		return "text"
	case KindBinary:
		return "binary"
	case KindDate:
		return "date"
	case KindTime:
		return "time"
	case KindTimestamp:
		return "timestamp"
	case KindTimestampTZ:
		return "timestamptz"
	case KindUUID:
		return "uuid"
	case KindJSON:
		return "json"
	default:
		return "unknown"
	}
}

// CanonicalType is a fully described, dialect-independent column type.
type CanonicalType struct {
	Kind TypeKind

	// Numeric attributes.
	BitWidth  int  // KindInt/KindFloat: 8, 16, 32, 64
	Signed    bool // KindInt
	Precision int  // KindDecimal
	Scale     int  // KindDecimal

	// Character/binary attributes.
	Length int  // KindString/KindBinary; 0 means unbounded
	Fixed  bool // CHAR/BINARY (true) vs VARCHAR/VARBINARY (false)

	// Native is the original source type string, kept for audit and for manual
	// review when Kind == KindUnknown.
	Native string

	// Lossy marks a mapping that may lose information (range, precision,
	// semantics). The reporter surfaces every lossy column for review.
	Lossy bool
}
