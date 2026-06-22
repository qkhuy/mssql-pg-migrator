// Package demo is a source adapter that returns a representative, hand-built
// schema without connecting to anything. It registers as the "demo" source so
// you can generate an assessment report end-to-end before the real database
// adapters are wired, and it doubles as a test fixture.
package demo

import (
	"context"
	"errors"

	"github.com/qkhuy/mssql-pg-migrator/internal/ir"
	"github.com/qkhuy/mssql-pg-migrator/internal/source"
)

func init() {
	source.Register("demo", func() source.Source { return &Source{} })
}

// Source implements source.Source with a fixed in-memory schema.
type Source struct{}

func (s *Source) Open(ctx context.Context, dsn string) error { return nil }
func (s *Source) Close() error                               { return nil }

func (s *Source) Read(ctx context.Context, t *ir.Table, r source.Range) (<-chan ir.Row, <-chan error) {
	rows := make(chan ir.Row)
	errs := make(chan error, 1)
	go func() {
		defer close(rows)
		defer close(errs)
		errs <- errors.New("demo source: data read not supported (assessment/schema only)")
	}()
	return rows, errs
}

// Introspect returns a representative SQL Server-style schema covering common
// types (including a GEOGRAPHY that has no clean mapping), a view, and routines.
func (s *Source) Introspect(ctx context.Context) (*ir.Schema, error) {
	intT := func(bits int) ir.CanonicalType { return ir.CanonicalType{Kind: ir.KindInt, BitWidth: bits, Signed: true} }

	customers := &ir.Table{
		Schema: "dbo", Name: "Customers", EstimatedRows: 500_000,
		Columns: []*ir.Column{
			{Name: "CustomerID", Type: withNative(intT(32), "int IDENTITY(1,1)"), IsIdentity: true},
			{Name: "FullName", Type: str(100, false, "nvarchar(100)"), Nullable: false},
			{Name: "Email", Type: str(256, false, "nvarchar(256)"), Nullable: true},
			{Name: "IsActive", Type: withNative(ir.CanonicalType{Kind: ir.KindBool}, "bit"), Nullable: false},
			{Name: "CreatedAt", Type: withNative(ir.CanonicalType{Kind: ir.KindTimestamp}, "datetime2"), Default: "now()"},
			{Name: "RowGuid", Type: withNative(ir.CanonicalType{Kind: ir.KindUUID}, "uniqueidentifier"), Nullable: false},
		},
		PrimaryKey: &ir.PrimaryKey{Name: "PK_Customers", Columns: []string{"CustomerID"}},
	}

	orders := &ir.Table{
		Schema: "dbo", Name: "Orders", EstimatedRows: 3_200_000,
		Columns: []*ir.Column{
			{Name: "OrderID", Type: withNative(intT(64), "bigint IDENTITY(1,1)"), IsIdentity: true},
			{Name: "CustomerID", Type: intT(32), Nullable: false},
			{Name: "Total", Type: dec(19, 4, "money")},
			{Name: "Notes", Type: withNative(ir.CanonicalType{Kind: ir.KindText}, "nvarchar(max)"), Nullable: true},
			{Name: "ShipLocation", Type: ir.CanonicalType{Kind: ir.KindUnknown, Native: "geography"}, Nullable: true},
			{Name: "PlacedAt", Type: withNative(ir.CanonicalType{Kind: ir.KindTimestampTZ}, "datetimeoffset"), Nullable: false},
		},
		PrimaryKey: &ir.PrimaryKey{Name: "PK_Orders", Columns: []string{"OrderID"}},
		ForeignKeys: []*ir.ForeignKey{{
			Name: "FK_Orders_Customers", Columns: []string{"CustomerID"},
			RefTable: "Customers", RefColumns: []string{"CustomerID"}, OnDelete: "NO ACTION",
		}},
	}

	orderItems := &ir.Table{
		Schema: "dbo", Name: "OrderItems", EstimatedRows: 11_800_000,
		Columns: []*ir.Column{
			{Name: "OrderItemID", Type: withNative(intT(64), "bigint IDENTITY(1,1)"), IsIdentity: true},
			{Name: "OrderID", Type: intT(64), Nullable: false},
			{Name: "Sku", Type: str(40, true, "char(40)"), Nullable: false},
			{Name: "Quantity", Type: withNative(intT(16), "smallint"), Nullable: false},
			{Name: "UnitPrice", Type: dec(19, 4, "money")},
		},
		PrimaryKey: &ir.PrimaryKey{Name: "PK_OrderItems", Columns: []string{"OrderItemID"}},
	}

	return &ir.Schema{
		Name:   "SourceDB",
		Tables: []*ir.Table{customers, orders, orderItems},
		Views: []*ir.View{
			{Schema: "dbo", Name: "vw_CustomerOrderTotals", Definition: "SELECT ... (T-SQL)"},
		},
		Routines: []*ir.Routine{
			{Schema: "dbo", Name: "sp_GetCustomerOrders", Kind: ir.RoutineProcedure, Definition: "CREATE PROCEDURE ... (T-SQL)"},
			{Schema: "dbo", Name: "fn_OrderTotal", Kind: ir.RoutineFunction, Definition: "CREATE FUNCTION ... (T-SQL)"},
			{Schema: "dbo", Name: "trg_Orders_Audit", Kind: ir.RoutineTrigger, Definition: "CREATE TRIGGER ... (T-SQL)"},
		},
	}, nil
}

func withNative(ct ir.CanonicalType, native string) ir.CanonicalType {
	ct.Native = native
	return ct
}

func str(length int, fixed bool, native string) ir.CanonicalType {
	return ir.CanonicalType{Kind: ir.KindString, Length: length, Fixed: fixed, Native: native}
}

func dec(p, s int, native string) ir.CanonicalType {
	return ir.CanonicalType{Kind: ir.KindDecimal, Precision: p, Scale: s, Native: native}
}
