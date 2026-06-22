package ir

// Schema is the canonical description of a database's structure.
type Schema struct {
	Name     string
	Tables   []*Table
	Views    []*View
	Routines []*Routine // stored procedures, functions, triggers
}

// Table describes one table and everything needed to recreate and load it.
type Table struct {
	Schema      string
	Name        string
	Columns     []*Column
	PrimaryKey  *PrimaryKey
	ForeignKeys []*ForeignKey
	Indexes     []*Index

	// EstimatedRows guides chunking and parallelism during data movement and
	// is shown as the data volume in the assessment report. It is an estimate
	// (e.g. from catalog statistics), not an exact count.
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

// View describes a view. Definition holds the source SQL, which a target
// adapter must translate to its own dialect (usually a manual-review step).
type View struct {
	Schema     string
	Name       string
	Definition string
}

// RoutineKind distinguishes stored procedures, functions, and triggers.
type RoutineKind int

const (
	RoutineProcedure RoutineKind = iota
	RoutineFunction
	RoutineTrigger
)

func (k RoutineKind) String() string {
	switch k {
	case RoutineProcedure:
		return "procedure"
	case RoutineFunction:
		return "function"
	case RoutineTrigger:
		return "trigger"
	default:
		return "routine"
	}
}

// Routine describes procedural code (procedure/function/trigger). Definition is
// the source dialect body; cross-dialect translation is a manual-review step.
type Routine struct {
	Schema     string
	Name       string
	Kind       RoutineKind
	Definition string
}

// Row is one record, with values ordered to match Table.Columns. Values use Go
// types produced by the source adapter; target adapters are responsible for
// encoding them for their bulk-load path.
type Row []any
