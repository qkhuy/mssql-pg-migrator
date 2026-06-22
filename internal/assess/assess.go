// Package assess builds a migration assessment: a complete, object-by-object
// plan of what will migrate and what it maps to (schema→schema, table→table,
// column→column, type→type, view→…, procedure→…), with data volumes and a
// per-object status. The report package renders it to Markdown/HTML.
package assess

import (
	"sort"
	"time"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/target"
)

// Status is the migration confidence for one object.
type Status int

const (
	StatusAuto        Status = iota // migrates automatically
	StatusReview                    // migrates but should be reviewed (lossy / dialect translation)
	StatusUnsupported               // no clean mapping; needs manual work
)

// Label is the human-readable (Vietnamese) status used in reports.
func (s Status) Label() string {
	switch s {
	case StatusAuto:
		return "Tự động"
	case StatusReview:
		return "Cần review"
	default:
		return "Không hỗ trợ"
	}
}

// Class is a CSS class hook for the HTML report.
func (s Status) Class() string {
	switch s {
	case StatusAuto:
		return "auto"
	case StatusReview:
		return "review"
	default:
		return "unsupported"
	}
}

// SchemaMapping is one source schema → target schema name pair.
type SchemaMapping struct {
	Source string
	Target string
}

// ColumnMapping captures column→column and type→type.
type ColumnMapping struct {
	Source     string // source column name
	Target     string // target column name
	SourceType string // source native type, e.g. "money"
	TargetType string // target native type, e.g. "numeric(19,4)"
	Status     Status
	Note       string
}

// TableMapping captures table→table plus its columns and data volume.
type TableMapping struct {
	Source        string // source schema.table
	Target        string // target schema.table
	EstimatedRows int64
	Status        Status // worst column status
	Columns       []ColumnMapping
}

// ObjectMapping captures views and routines (procedures/functions/triggers).
type ObjectMapping struct {
	Kind   string // "view", "procedure", "function", "trigger"
	Source string
	Target string
	Status Status
	Note   string
}

// Summary holds the headline numbers.
type Summary struct {
	TotalTables   int
	TotalColumns  int
	TotalViews    int
	TotalRoutines int
	TotalRows     int64

	AutoObjects        int
	ReviewObjects      int
	UnsupportedObjects int
	AutoPercent        int
}

// Assessment is the full, renderable plan.
type Assessment struct {
	SourceEngine string
	TargetEngine string
	GeneratedAt  time.Time
	Schemas      []SchemaMapping
	Summary      Summary
	Tables       []TableMapping
	Views        []ObjectMapping
	Routines     []ObjectMapping
}

// Build computes the assessment by mapping a source schema through a target's
// pure mapping logic. No database connection is required.
func Build(srcEngine, dstEngine string, s *ir.Schema, m target.Mapper) *Assessment {
	a := &Assessment{
		SourceEngine: srcEngine,
		TargetEngine: dstEngine,
		GeneratedAt:  time.Now(),
	}
	schemas := map[string]bool{}

	for _, t := range s.Tables {
		schemas[t.Schema] = true
		tm := TableMapping{
			Source:        qn(t.Schema, t.Name),
			Target:        qn(m.MapIdentifier(t.Schema), m.MapIdentifier(t.Name)),
			EstimatedRows: t.EstimatedRows,
			Status:        StatusAuto,
		}
		for _, c := range t.Columns {
			mp := m.MapType(c.Type)
			st := StatusAuto
			switch {
			case c.Type.Kind == ir.KindUnknown:
				st = StatusUnsupported
			case mp.Lossy:
				st = StatusReview
			}
			if st > tm.Status {
				tm.Status = st
			}
			// The headline percentage is counted over columns (leaf objects),
			// not tables, so one bad column doesn't sink a whole table's number.
			tally(&a.Summary, st)
			tm.Columns = append(tm.Columns, ColumnMapping{
				Source:     c.Name,
				Target:     m.MapIdentifier(c.Name),
				SourceType: sourceType(c.Type),
				TargetType: mp.Native,
				Status:     st,
				Note:       mp.Note,
			})
		}
		a.Summary.TotalColumns += len(tm.Columns)
		a.Summary.TotalRows += t.EstimatedRows
		a.Tables = append(a.Tables, tm)
	}

	for _, v := range s.Views {
		schemas[v.Schema] = true
		a.Views = append(a.Views, ObjectMapping{
			Kind:   "view",
			Source: qn(v.Schema, v.Name),
			Target: qn(m.MapIdentifier(v.Schema), m.MapIdentifier(v.Name)),
			Status: StatusReview,
			Note:   "Định nghĩa view cần dịch & review thủ công",
		})
		tally(&a.Summary, StatusReview)
	}

	for _, r := range s.Routines {
		schemas[r.Schema] = true
		a.Routines = append(a.Routines, ObjectMapping{
			Kind:   r.Kind.String(),
			Source: qn(r.Schema, r.Name),
			Target: qn(m.MapIdentifier(r.Schema), m.MapIdentifier(r.Name)),
			Status: StatusReview,
			Note:   "Logic cần dịch sang dialect đích & review thủ công",
		})
		tally(&a.Summary, StatusReview)
	}

	for sc := range schemas {
		a.Schemas = append(a.Schemas, SchemaMapping{Source: sc, Target: m.MapIdentifier(sc)})
	}
	sort.Slice(a.Schemas, func(i, j int) bool { return a.Schemas[i].Source < a.Schemas[j].Source })

	a.Summary.TotalTables = len(s.Tables)
	a.Summary.TotalViews = len(s.Views)
	a.Summary.TotalRoutines = len(s.Routines)
	if total := a.Summary.AutoObjects + a.Summary.ReviewObjects + a.Summary.UnsupportedObjects; total > 0 {
		a.Summary.AutoPercent = a.Summary.AutoObjects * 100 / total
	}
	return a
}

func tally(s *Summary, st Status) {
	switch st {
	case StatusAuto:
		s.AutoObjects++
	case StatusReview:
		s.ReviewObjects++
	default:
		s.UnsupportedObjects++
	}
}

func qn(schema, name string) string {
	if schema == "" {
		return name
	}
	return schema + "." + name
}

// sourceType renders a column's source-side type for display, preferring the
// native string captured by the source adapter.
func sourceType(ct ir.CanonicalType) string {
	if ct.Native != "" {
		return ct.Native
	}
	return ct.Kind.String()
}
