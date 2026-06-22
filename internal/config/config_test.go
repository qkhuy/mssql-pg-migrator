package config

import "testing"

func TestParseYAML(t *testing.T) {
	in := []byte(`
source:
  engine: mssql
  dsn: sqlserver://h/db
target:
  engine: postgres
  dsn: postgres://h/db
migration:
  dry_run: true
  parallelism: 8
  tables: [Orders, Customers]
`)
	c, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if c.Source.Engine != "mssql" || c.Target.Engine != "postgres" {
		t.Errorf("engines = %s/%s", c.Source.Engine, c.Target.Engine)
	}
	if !c.Migration.DryRun || c.Migration.Parallelism != 8 || len(c.Migration.Tables) != 2 {
		t.Errorf("migration = %+v", c.Migration)
	}
}

func TestParseJSONCompat(t *testing.T) {
	// YAML is a superset of JSON, so JSON configs still parse.
	in := []byte(`{"source":{"engine":"demo","dsn":""},"target":{"engine":"postgres","dsn":""},"migration":{"parallelism":4}}`)
	c, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if c.Source.Engine != "demo" || c.Migration.Parallelism != 4 {
		t.Errorf("unexpected: %+v", c)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	c := &Config{
		Source:    Endpoint{Engine: "mssql", DSN: "a"},
		Target:    Endpoint{Engine: "postgres", DSN: "b"},
		Migration: Migration{Parallelism: 4, Tables: []string{"T1"}},
	}
	b, err := Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Engine != "mssql" || got.Target.DSN != "b" || got.Migration.Tables[0] != "T1" {
		t.Errorf("round trip mismatch: %+v", got)
	}
}
