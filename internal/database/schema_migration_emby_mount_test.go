package database

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/model"
)

func TestEnsureEmbyMountsCompatibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Create a table without sort_order simulating an older schema
	if err := db.Exec(`CREATE TABLE emby_mounts (
		id varchar(36) PRIMARY KEY,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime,
		account_id text,
		remote_view_id text,
		remote_view_name text,
		collection_type text,
		name text,
		proxy_play numeric DEFAULT false,
		enabled numeric DEFAULT true
	)`).Error; err != nil {
		t.Fatal(err)
	}

	// Insert older rows
	now := time.Now()
	_ = db.Exec("INSERT INTO emby_mounts (id, name, created_at) VALUES (?, ?, ?)", "m1", "Mount 1", now.Add(-2*time.Hour)).Error
	_ = db.Exec("INSERT INTO emby_mounts (id, name, created_at) VALUES (?, ?, ?)", "m2", "Mount 2", now.Add(-1*time.Hour)).Error

	// Run compatibility migration
	if err := ensureEmbyMountsCompatibility(db); err != nil {
		t.Fatalf("ensureEmbyMountsCompatibility failed: %v", err)
	}

	// Verify column sort_order exists and values are initialized sequentially
	if !db.Migrator().HasColumn(&model.EmbyMount{}, "sort_order") {
		t.Fatal("expected sort_order column to be added")
	}

	var m1, m2 model.EmbyMount
	if err := db.Where("id = ?", "m1").First(&m1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", "m2").First(&m2).Error; err != nil {
		t.Fatal(err)
	}

	if m1.SortOrder != 0 || m2.SortOrder != 1 {
		t.Fatalf("unexpected sort orders: m1=%d, m2=%d", m1.SortOrder, m2.SortOrder)
	}
}
