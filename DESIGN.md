# Database Migration Tool — Design

A lightweight, high-performance tool for migrating between relational databases.
Built **engine-agnostic from the start**: it is not limited to MSSQL → PostgreSQL.
New source and target engines plug in without touching the core.

## Why a canonical intermediate representation (IR)

Writing a converter for every `(source, target)` pair scales as **N × M**. Adding
one database means writing many new converters. Instead, every source translates
into one canonical IR, and every target translates out of it:

```
   Sources                  Canonical IR (internal/ir)          Targets
  ┌────────┐               ┌──────────────────────────┐        ┌──────────┐
  │ MSSQL  │──Introspect─▶ │  Schema / Table / Column  │ ─DDL─▶ │ Postgres │
  │ MySQL  │──Introspect─▶ │  CanonicalType            │ ─DDL─▶ │ MySQL    │
  │ Oracle │──Introspect─▶ │  Row stream               │ ─DDL─▶ │ ...      │
  └────────┘               └──────────────────────────┘        └──────────┘
```

Cost is **N + M** adapters. A new engine is one new package and one import line.

## Package layout

| Package | Responsibility |
|---------|----------------|
| `internal/ir` | Canonical, dialect-independent schema + type model + `Row` |
| `internal/source` | `Source` interface + name registry |
| `internal/target` | `Target` interface + name registry + `Warning` |
| `internal/source/<engine>` | One source adapter; registers via `init()` |
| `internal/target/<engine>` | One target adapter; registers via `init()` |
| `internal/pipeline` | Engine-agnostic orchestration (introspect → schema → data → validate) |
| `internal/config` | Configuration loading |
| `cmd/migrator` | CLI; blank-imports the adapters to register them |

The pipeline depends only on the `source.Source` and `target.Target` interfaces,
so any registered pair works with no pipeline changes.

## Core contracts

- **`source.Source`**: `Open`, `Close`, `Introspect() *ir.Schema`,
  `Read(table, Range) -> (<-chan Row, <-chan error)`. Read-only — never mutates
  the source. `Range` chunks by primary key for parallel, resumable reads.
- **`target.Target`**: `Open`, `Close`, `RenderDDL` (no execution — backs
  dry-run and review), `ApplySchema`, `BulkLoad` (fastest engine path, e.g.
  PostgreSQL `COPY`).

## Correctness principles

- **Deterministic core.** Schema mapping, type mapping, and data movement are
  rule-based and repeatable — no AI in the lib path (a future opt-in
  enhancement, never the source of truth).
- **Never silently guess.** Unmapped types become `ir.KindUnknown` carrying the
  native type; lossy mappings set `Lossy`. Targets emit a `Warning` per
  questionable construct. Everything surfaces in the report for human review.
- **Read-only source.** Source adapters must not write to the source database.

## Performance plan (millions–tens of millions of rows)

The bottleneck is I/O, not CPU. The design targets it directly:

1. **Bulk-load protocol** on the target (PostgreSQL `COPY` via `pgx.CopyFrom`),
   not row-by-row INSERT.
2. **Range-chunked parallel reads** by primary key (not `OFFSET`), up to
   `Parallelism` chunks concurrently, with bounded channels for backpressure.
3. **Bounded memory** — stream rows, never materialize a whole table; reuse
   buffers to keep GC pressure low.
4. **Defer indexes/FKs/triggers** until after bulk load, then rebuild.
5. **Resumable checkpoints** — record the last committed PK per chunk.

## Adding a new engine

1. Create `internal/source/<engine>` (or `internal/target/<engine>`).
2. Implement the interface; call `source.Register` / `target.Register` in `init()`.
3. Add a blank import in `cmd/migrator/main.go`.

No other code changes are required — all existing engines on the other side
immediately interoperate with the new one.

## Status

Skeleton: interfaces, registries, IR, pipeline flow, CLI, and stub `mssql`
source + `postgres` target are in place and compile. Next: implement the MSSQL
introspection + read path and the PostgreSQL DDL generation + `COPY` load.
