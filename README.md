# migrator

A lightweight, high-performance database migration tool. It assesses, plans,
and moves a database from one engine to another — starting with **SQL Server →
PostgreSQL**, but built engine-agnostic so new source and target databases plug
in without touching the core.

- **Read-only assessment** with a visual HTML/Markdown report: see exactly what
  migrates and what it maps to (schema→schema, table→table, column→column,
  **type→type**, view→…, procedure→…) plus data volumes and a color-coded
  status — *before* you commit.
- **High-throughput data movement** via the target's fastest bulk path
  (PostgreSQL `COPY`), streamed with bounded memory and parallel across tables.
- **Resumable**: a checkpoint lets an interrupted run continue where it stopped.
- **Validated**: source vs target row-count check after load.
- **Safe**: the source is only ever read; lossy/unsupported constructs are
  surfaced for review, never silently guessed.

## Install

```bash
go install github.com/qkhuy/mssql-pg-migrator/cmd/migrator@latest
# or build from source:
git clone https://github.com/qkhuy/mssql-pg-migrator && cd mssql-pg-migrator
go build -o migrator ./cmd/migrator
```

## Configuration

A JSON file describes the source, target, and run options:

```json
{
  "source": { "engine": "mssql",    "dsn": "sqlserver://user:pass@host:1433?database=SourceDB" },
  "target": { "engine": "postgres", "dsn": "postgres://user:pass@host:5432/targetdb?sslmode=require" },
  "migration": { "dry_run": false, "parallelism": 4, "tables": [] }
}
```

`tables: []` migrates everything; list names to migrate a subset. Keep secrets
out of the file where possible (e.g. inject the DSN from the environment).

## Workflow

```
assess  →  plan  →  run
(report)  (dry)    (execute + validate)
```

```bash
# 1) Read-only assessment report (open the HTML in a browser)
migrator assess -config config.json -format html -out assessment.html

# 2) Dry-run: see the DDL and table plan, write nothing
migrator plan -config config.json

# 3) Execute: schema → data (COPY) → indexes/FKs/sequences → validate
migrator run -config config.json
```

`run` writes a checkpoint (`.migrator-state.json` by default, `-state` to
change). If it fails partway, rerun the same command to resume — completed
tables are skipped.

Other commands: `migrator engines` (list plug-in engines), `migrator version`.

## How migration works

1. **Introspect** the source into a canonical, engine-independent model
   (tables, columns, types, PK/FK, indexes, views, routines, row estimates).
2. **Apply schema**: create target schemas, tables, and primary keys. Secondary
   indexes and foreign keys are deferred for load speed.
3. **Load data**: stream each table through the target's bulk path (PostgreSQL
   `COPY`), several tables in parallel.
4. **Finalize**: create indexes and foreign keys, and advance identity
   sequences past the loaded data.
5. **Validate**: compare source and target row counts per table.

Procedures, functions, triggers, and view bodies are reported as
manual-review items — cross-dialect logic translation is not auto-applied (a
planned, opt-in enhancement).

## Desktop UI (Wails)

A desktop GUI lives in [`ui/`](ui/) — a [Wails](https://wails.io) app (Go +
Svelte) that binds the **same** `internal/app` service the CLI uses, so both
surfaces behave identically. It provides the connection screen, the assessment
view (table/column/type mappings with status badges), dry-run, and a live
per-table progress view during `run`.

It is a separate Go module so the CLI/core build stays independent of the GUI
toolchain. Build it with the Wails CLI:

```bash
cd ui && wails dev      # or: wails build
```

See [ui/README.md](ui/README.md) for prerequisites.

## Extending to other engines

The tool uses a canonical intermediate representation, so adding an engine is
**one new package + one import line** — every engine on the other side then
interoperates. See [DESIGN.md](DESIGN.md).

- New **source**: implement `source.Source` (+ optional `source.Counter`),
  register it in `init()`, add a blank import in `cmd/migrator/main.go`.
- New **target**: implement `target.Target` (+ `target.Mapper`,
  `target.Counter`).

## Status (v0.1.0)

Implemented and unit-tested: the canonical model, the assessment report, the
SQL Server introspection, PostgreSQL type mapping / DDL / `COPY` load,
parallel pipeline, checkpointing, and validation.

A live end-to-end round trip is covered by an integration test gated behind the
`integration` build tag; run it against your own databases:

```bash
MIGRATOR_MSSQL_DSN=... MIGRATOR_PG_DSN=... go test -tags integration ./internal/integration/
```

Known limitations in v0.1.0: intra-table parallelism is table-level (very large
single tables stream sequentially); value coercion for some SQL Server types
(e.g. `uniqueidentifier`, spatial types) and cross-dialect view/routine bodies
need review. These are tracked for upcoming releases.

## License

MIT — see [LICENSE](LICENSE).
