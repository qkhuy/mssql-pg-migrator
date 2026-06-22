package ir

// Schema is the canonical description of a database's structure.
type Schema struct {
	Name   string
	Tables []*Table
}

// Table describes one table and everything needed to recreate and load it.
type Table struct {
	Schema      string
	Name        string
	Columns     []*Column
	PrimaryKey  *PrimaryKey
	ForeignKeys []*ForeignKey
	Indexes     []*Index

	// EstimatedRows guides chunking and parallelism during data movement.
	// It is an estimate (e.g. from catalog statistics), not an exact count.
	EstimatedRows int64
}

// Column describes one column in canonical terms.
type Column struct {
	Name     string
	Type     CanonicalType
	Nullable bool

	// Default is a canonicalized default expression, or "" if none. Adapters
	// translate common functions (e.g. GETDATE() -> now()); anything not
	// recognized is preserved verbatim and flagged for review.
	Default string

	// Identity / auto-increment metadata. Targets map this to their own
	// mechanism (IDENTITY, SERIAL, GENERATED ... AS IDENTITY, AUTO_INCREMENT).
	IsIdentity   bool
	IdentitySeed int64
	IdentityStep int64
}

// PrimaryKey identifies the columns forming the primary key. The key columns
// also drive range-based chunking for parallel, resumable reads.
type PrimaryKey struct {
	Name    string
	Columns []string
}

// ForeignKey describes a referential constraint.
type ForeignKey struct {
	Name       string
	Columns    []string
	RefTable   string
	RefColumns []string
	OnDelete   string // canonical action: "CASCADE", "SET NULL", "RESTRICT", "NO ACTION", ""
	OnUpdate   string
}

// Index describes a secondary index.
type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

// Row is one record, with values ordered to match Table.Columns. Values use Go
// types produced by the source adapter; target adapters are responsible for
// encoding them for their bulk-load path.
type Row []any
