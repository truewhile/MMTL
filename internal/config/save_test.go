package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveDatabaseConfig(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	dsn := "postgres://admin:pass@127.0.0.1:5432/mebox?sslmode=disable"
	if err := SaveDatabaseConfig("postgres", dsn); err != nil {
		t.Fatalf("SaveDatabaseConfig error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("expected config.yaml to exist: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Database.Type != "postgres" {
		t.Fatalf("expected database.type=postgres, got %s", loaded.Database.Type)
	}
	if loaded.Database.DSN != dsn {
		t.Fatalf("expected dsn=%s, got %s", dsn, loaded.Database.DSN)
	}
}
