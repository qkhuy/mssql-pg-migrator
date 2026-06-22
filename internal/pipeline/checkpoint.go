package pipeline

import (
	"encoding/json"
	"os"
	"sync"
)

// checkpoint persists migration progress so a run can resume after a failure or
// interruption. It is intentionally coarse (table granularity): each table is
// loaded inside a single target transaction, so a table is either fully
// committed (recorded done) or rolled back (retried on resume).
type checkpoint struct {
	mu sync.Mutex

	path          string
	SchemaApplied bool            `json:"schema_applied"`
	Done          map[string]bool `json:"done"` // key: schema.table
}

func loadCheckpoint(path string) (*checkpoint, error) {
	cp := &checkpoint{path: path, Done: map[string]bool{}}
	if path == "" {
		return cp, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cp, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, cp); err != nil {
		return nil, err
	}
	if cp.Done == nil {
		cp.Done = map[string]bool{}
	}
	cp.path = path
	return cp, nil
}

func (c *checkpoint) save() error {
	if c.path == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0o644)
}

func (c *checkpoint) isDone(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Done[key]
}

func (c *checkpoint) markDone(key string) {
	c.mu.Lock()
	c.Done[key] = true
	c.mu.Unlock()
	_ = c.save()
}

func (c *checkpoint) markSchemaApplied() {
	c.mu.Lock()
	c.SchemaApplied = true
	c.mu.Unlock()
	_ = c.save()
}

// clear removes the checkpoint file (called on a fully successful run).
func (c *checkpoint) clear() {
	if c.path != "" {
		_ = os.Remove(c.path)
	}
}
