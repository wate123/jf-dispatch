package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndEnvOverride(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "c.yaml")
	if e := os.WriteFile(p, []byte("version: 1\nworker:\n  max_jobs: 3\n  scheduler: home-server:7000\n"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("JF_WORKER_MAX_JOBS", "4")
	c, e := Load(p)
	if e != nil {
		t.Fatal(e)
	}
	if c.Worker.MaxJobs != 4 || c.Worker.Scheduler != "home-server:7000" {
		t.Fatalf("unexpected: %+v", c.Worker)
	}
}
