package service

import (
	"path/filepath"
	"testing"

	"github.com/ShukeBta/MMTL/internal/config"
)

func TestBackupFilePathRejectsTraversal(t *testing.T) {
	svc := &BackupService{cfg: &config.Config{}}
	svc.cfg.App.DataDir = t.TempDir()
	for _, name := range []string{"../evil.db", `..\evil.db`, "nested/evil.db", "evil.sqlite"} {
		if _, err := svc.backupFilePath(name); err == nil {
			t.Fatalf("backupFilePath(%q) allowed traversal or non-backup file", name)
		}
	}
	path, err := svc.backupFilePath("mmtl_20260611_010203.db")
	if err != nil {
		t.Fatalf("backupFilePath(valid) = %v", err)
	}
	if filepath.Dir(path) != filepath.Join(svc.cfg.App.DataDir, "backups") {
		t.Fatalf("backupFilePath(valid) dir = %q", filepath.Dir(path))
	}
}

func TestSafeZipTargetRejectsZipSlip(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../evil.exe", `..\evil.exe`, "/tmp/evil.exe"} {
		if _, err := safeZipTarget(root, name); err == nil {
			t.Fatalf("safeZipTarget(%q) allowed zip-slip path", name)
		}
	}
	if _, err := safeZipTarget(root, "ffmpeg/bin/ffmpeg.exe"); err != nil {
		t.Fatalf("safeZipTarget(valid) = %v", err)
	}
}
