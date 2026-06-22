# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-06-22

First release. SQL Server → PostgreSQL migration on an engine-agnostic core.

### Added
- Canonical intermediate representation (schema, types, views, routines) so
  sources and targets plug in independently (N+M, not N×M).
- `assess` command: read-only visual report (HTML + Markdown) of the full plan
  — schema→schema, table→table, column→column, type→type, view→…, routine→…,
  data volumes, and a color-coded status (auto / review / unsupported).
- SQL Server source adapter: introspection (tables, columns, PK, FK, indexes,
  views, routines, row counts) and streaming reads.
- PostgreSQL target adapter: deterministic type mapping, DDL generation,
  `COPY`-based bulk load, and finalize (indexes, foreign keys, identity
  sequence resets).
- Pipeline: parallel table loading, checkpoint-based resume, row-count
  validation.
- CLI: `assess`, `plan` (dry-run), `run`, `engines`, `version`.
- Unit tests for type mapping, DDL rendering, assessment, and reporting;
  integration test gated behind the `integration` build tag.

### Known limitations
- Intra-table parallelism is table-level; very large single tables stream
  sequentially.
- Value coercion for some SQL Server types (e.g. `uniqueidentifier`, spatial)
  and cross-dialect view/routine bodies require manual review.
