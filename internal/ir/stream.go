package ir

// RowStream is a pull-based cursor over a table's rows. It is the data-movement
// contract between a source (which produces a stream) and a target (which
// consumes it, e.g. feeding PostgreSQL COPY).
//
// Pull-based is deliberate: the target drives the read, and a mid-stream source
// error is surfaced via Err() *after* Next() returns false, so the target can
// abort the load instead of committing a truncated copy.
//
// Usage:
//
//	defer s.Close()
//	for s.Next() {
//	    row := s.Row()
//	}
//	if err := s.Err(); err != nil { ... }
type RowStream interface {
	// Next advances to the next row. It returns false at end of stream or on
	// error; check Err() afterwards to distinguish the two.
	Next() bool
	// Row returns the current row. Valid only after Next() returned true.
	Row() Row
	// Err returns the first error encountered, or nil on clean completion.
	Err() error
	// Close releases underlying resources.
	Close() error
}

// ErrStream is a RowStream that yields no rows and reports err. Useful for
// adapters that cannot stream a given table.
type ErrStream struct{ E error }

func (s ErrStream) Next() bool   { return false }
func (s ErrStream) Row() Row     { return nil }
func (s ErrStream) Err() error   { return s.E }
func (s ErrStream) Close() error { return nil }
