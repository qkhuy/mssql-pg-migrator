// Package mssql is the Microsoft SQL Server source adapter. It registers itself
// as the "mssql" source engine on import and uses the go-mssqldb driver.
//
// It is read-only: it never issues a statement that mutates the source.
package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
)

func init() {
	source.Register("mssql", func() source.Source { return &Source{} })
}

// Source implements source.Source (and source.Counter) for SQL Server.
type Source struct {
	db *sql.DB
}

func (s *Source) Open(ctx context.Context, dsn string) error {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("ping: %w", err)
	}
	s.db = db
	return nil
}

func (s *Source) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Count returns the exact row count for validation.
func (s *Source) Count(ctx context.Context, t *ir.Table) (int64, error) {
	var n int64
	q := fmt.Sprintf("SELECT COUNT_BIG(*) FROM %s", quote(t.Schema, t.Name))
	err := s.db.QueryRowContext(ctx, q).Scan(&n)
	return n, err
}

// Read streams every row of a table. Range is reserved for future intra-table
// chunking; v0.1 streams the whole table (memory stays bounded — rows are
// pulled one at a time straight into the target's COPY).
func (s *Source) Read(ctx context.Context, t *ir.Table, r source.Range) (ir.RowStream, error) {
	cols := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = bracket(c.Name)
	}
	q := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), quote(t.Schema, t.Name))
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	return &rowStream{rows: rows, n: len(t.Columns)}, nil
}

// rowStream adapts *sql.Rows to ir.RowStream.
type rowStream struct {
	rows *sql.Rows
	n    int
	cur  ir.Row
	err  error
}

func (s *rowStream) Next() bool {
	if !s.rows.Next() {
		s.err = s.rows.Err()
		return false
	}
	vals := make([]any, s.n)
	ptrs := make([]any, s.n)
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := s.rows.Scan(ptrs...); err != nil {
		s.err = err
		return false
	}
	s.cur = ir.Row(vals)
	return true
}

func (s *rowStream) Row() ir.Row  { return s.cur }
func (s *rowStream) Err() error   { return s.err }
func (s *rowStream) Close() error { return s.rows.Close() }

// --- introspection -------------------------------------------------------

func (s *Source) Introspect(ctx context.Context) (*ir.Schema, error) {
	schema := &ir.Schema{Name: "source"}
	byKey := map[string]*ir.Table{}

	if err := s.loadTables(ctx, schema, byKey); err != nil {
		return nil, fmt.Errorf("tables: %w", err)
	}
	if err := s.loadColumns(ctx, byKey); err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}
	if err := s.loadPrimaryKeys(ctx, byKey); err != nil {
		return nil, fmt.Errorf("primary keys: %w", err)
	}
	if err := s.loadIndexes(ctx, byKey); err != nil {
		return nil, fmt.Errorf("indexes: %w", err)
	}
	if err := s.loadForeignKeys(ctx, byKey); err != nil {
		return nil, fmt.Errorf("foreign keys: %w", err)
	}
	if err := s.loadRowCounts(ctx, byKey); err != nil {
		return nil, fmt.Errorf("row counts: %w", err)
	}
	if err := s.loadViews(ctx, schema); err != nil {
		return nil, fmt.Errorf("views: %w", err)
	}
	if err := s.loadRoutines(ctx, schema); err != nil {
		return nil, fmt.Errorf("routines: %w", err)
	}
	return schema, nil
}

func key(schema, name string) string { return schema + "." + name }

func (s *Source) loadTables(ctx context.Context, schema *ir.Schema, byKey map[string]*ir.Table) error {
	const q = `SELECT s.name, t.name
		FROM sys.tables t
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE t.is_ms_shipped = 0
		ORDER BY s.name, t.name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name string
		if err := rows.Scan(&sch, &name); err != nil {
			return err
		}
		t := &ir.Table{Schema: sch, Name: name}
		byKey[key(sch, name)] = t
		schema.Tables = append(schema.Tables, t)
	}
	return rows.Err()
}

func (s *Source) loadColumns(ctx context.Context, byKey map[string]*ir.Table) error {
	const q = `SELECT s.name, t.name, c.name, ty.name, c.max_length, c.precision, c.scale, c.is_nullable, c.is_identity
		FROM sys.columns c
		JOIN sys.tables t ON c.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		JOIN sys.types ty ON c.user_type_id = ty.user_type_id
		WHERE t.is_ms_shipped = 0
		ORDER BY s.name, t.name, c.column_id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, tbl, col, ty string
		var maxLen, prec, scale int
		var nullable, identity bool
		if err := rows.Scan(&sch, &tbl, &col, &ty, &maxLen, &prec, &scale, &nullable, &identity); err != nil {
			return err
		}
		t := byKey[key(sch, tbl)]
		if t == nil {
			continue
		}
		t.Columns = append(t.Columns, &ir.Column{
			Name:       col,
			Type:       mapColumnType(ty, maxLen, prec, scale),
			Nullable:   nullable,
			IsIdentity: identity,
		})
	}
	return rows.Err()
}

func (s *Source) loadPrimaryKeys(ctx context.Context, byKey map[string]*ir.Table) error {
	const q = `SELECT s.name, t.name, i.name, c.name
		FROM sys.indexes i
		JOIN sys.tables t ON i.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE i.is_primary_key = 1
		ORDER BY s.name, t.name, ic.key_ordinal`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, tbl, idx, col string
		if err := rows.Scan(&sch, &tbl, &idx, &col); err != nil {
			return err
		}
		t := byKey[key(sch, tbl)]
		if t == nil {
			continue
		}
		if t.PrimaryKey == nil {
			t.PrimaryKey = &ir.PrimaryKey{Name: idx}
		}
		t.PrimaryKey.Columns = append(t.PrimaryKey.Columns, col)
	}
	return rows.Err()
}

func (s *Source) loadIndexes(ctx context.Context, byKey map[string]*ir.Table) error {
	const q = `SELECT s.name, t.name, i.name, i.is_unique, c.name
		FROM sys.indexes i
		JOIN sys.tables t ON i.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE i.is_primary_key = 0 AND i.is_unique_constraint = 0 AND i.type > 0 AND ic.is_included_column = 0
		ORDER BY s.name, t.name, i.name, ic.key_ordinal`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := map[string]*ir.Index{}
	for rows.Next() {
		var sch, tbl, idx, col string
		var unique bool
		if err := rows.Scan(&sch, &tbl, &idx, &unique, &col); err != nil {
			return err
		}
		t := byKey[key(sch, tbl)]
		if t == nil {
			continue
		}
		ik := key(sch, tbl) + "/" + idx
		ix := seen[ik]
		if ix == nil {
			ix = &ir.Index{Name: idx, Unique: unique}
			seen[ik] = ix
			t.Indexes = append(t.Indexes, ix)
		}
		ix.Columns = append(ix.Columns, col)
	}
	return rows.Err()
}

func (s *Source) loadForeignKeys(ctx context.Context, byKey map[string]*ir.Table) error {
	const q = `SELECT fk.name, ps.name, pt.name, pc.name, rs.name, rt.name, rc.name,
			fk.delete_referential_action_desc, fk.update_referential_action_desc
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		JOIN sys.tables pt ON fkc.parent_object_id = pt.object_id
		JOIN sys.schemas ps ON pt.schema_id = ps.schema_id
		JOIN sys.columns pc ON fkc.parent_object_id = pc.object_id AND fkc.parent_column_id = pc.column_id
		JOIN sys.tables rt ON fkc.referenced_object_id = rt.object_id
		JOIN sys.schemas rs ON rt.schema_id = rs.schema_id
		JOIN sys.columns rc ON fkc.referenced_object_id = rc.object_id AND fkc.referenced_column_id = rc.column_id
		ORDER BY fk.name, fkc.constraint_column_id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := map[string]*ir.ForeignKey{}
	for rows.Next() {
		var name, psch, ptbl, pcol, rsch, rtbl, rcol, del, upd string
		if err := rows.Scan(&name, &psch, &ptbl, &pcol, &rsch, &rtbl, &rcol, &del, &upd); err != nil {
			return err
		}
		t := byKey[key(psch, ptbl)]
		if t == nil {
			continue
		}
		fk := seen[name]
		if fk == nil {
			fk = &ir.ForeignKey{
				Name:     name,
				RefTable: key(rsch, rtbl), // qualified
				OnDelete: normalizeAction(del),
				OnUpdate: normalizeAction(upd),
			}
			seen[name] = fk
			t.ForeignKeys = append(t.ForeignKeys, fk)
		}
		fk.Columns = append(fk.Columns, pcol)
		fk.RefColumns = append(fk.RefColumns, rcol)
	}
	return rows.Err()
}

func (s *Source) loadRowCounts(ctx context.Context, byKey map[string]*ir.Table) error {
	const q = `SELECT s.name, t.name, SUM(ps.row_count)
		FROM sys.dm_db_partition_stats ps
		JOIN sys.tables t ON ps.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE ps.index_id IN (0, 1)
		GROUP BY s.name, t.name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, tbl string
		var n int64
		if err := rows.Scan(&sch, &tbl, &n); err != nil {
			return err
		}
		if t := byKey[key(sch, tbl)]; t != nil {
			t.EstimatedRows = n
		}
	}
	return rows.Err()
}

func (s *Source) loadViews(ctx context.Context, schema *ir.Schema) error {
	const q = `SELECT s.name, v.name, OBJECT_DEFINITION(v.object_id)
		FROM sys.views v
		JOIN sys.schemas s ON v.schema_id = s.schema_id
		WHERE v.is_ms_shipped = 0
		ORDER BY s.name, v.name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name string
		var def sql.NullString
		if err := rows.Scan(&sch, &name, &def); err != nil {
			return err
		}
		schema.Views = append(schema.Views, &ir.View{Schema: sch, Name: name, Definition: def.String})
	}
	return rows.Err()
}

func (s *Source) loadRoutines(ctx context.Context, schema *ir.Schema) error {
	const q = `SELECT s.name, o.name, o.type, OBJECT_DEFINITION(o.object_id)
		FROM sys.objects o
		JOIN sys.schemas s ON o.schema_id = s.schema_id
		WHERE o.type IN ('P','FN','IF','TF','TR') AND o.is_ms_shipped = 0
		ORDER BY s.name, o.name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name, typ string
		var def sql.NullString
		if err := rows.Scan(&sch, &name, &typ, &def); err != nil {
			return err
		}
		schema.Routines = append(schema.Routines, &ir.Routine{
			Schema: sch, Name: name, Kind: routineKind(typ), Definition: def.String,
		})
	}
	return rows.Err()
}

func normalizeAction(desc string) string {
	switch strings.ToUpper(strings.TrimSpace(desc)) {
	case "CASCADE":
		return "CASCADE"
	case "SET_NULL":
		return "SET NULL"
	case "SET_DEFAULT":
		return "SET DEFAULT"
	default:
		return "NO ACTION"
	}
}

func bracket(name string) string { return "[" + strings.ReplaceAll(name, "]", "]]") + "]" }

func quote(schema, name string) string {
	if schema == "" {
		return bracket(name)
	}
	return bracket(schema) + "." + bracket(name)
}
