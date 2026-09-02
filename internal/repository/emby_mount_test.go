package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/database"
	"github.com/truewhile/MeBox/internal/model"
)

func TestEmbyMountSortOrderAndReorder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repos := New(db)
	ctx := t.Context()

	// 1. Create mounts and verify auto-assigned sort_order
	m1 := &model.EmbyMount{AccountID: "acct-1", RemoteViewID: "view-1", Name: "Mount 1"}
	m2 := &model.EmbyMount{AccountID: "acct-1", RemoteViewID: "view-2", Name: "Mount 2"}
	m3 := &model.EmbyMount{AccountID: "acct-1", RemoteViewID: "view-3", Name: "Mount 3"}

	if err := repos.EmbyMount.Create(ctx, m1); err != nil {
		t.Fatalf("create m1: %v", err)
	}
	if err := repos.EmbyMount.Create(ctx, m2); err != nil {
		t.Fatalf("create m2: %v", err)
	}
	if err := repos.EmbyMount.Create(ctx, m3); err != nil {
		t.Fatalf("create m3: %v", err)
	}

	if m1.SortOrder >= m2.SortOrder || m2.SortOrder >= m3.SortOrder {
		t.Fatalf("expected ascending sort order on create: m1=%d, m2=%d, m3=%d",
			m1.SortOrder, m2.SortOrder, m3.SortOrder)
	}

	// 2. Query list and verify initial order
	list, err := repos.EmbyMount.List(ctx)
	if err != nil {
		t.Fatalf("list mounts: %v", err)
	}
	if len(list) != 3 || list[0].ID != m1.ID || list[1].ID != m2.ID || list[2].ID != m3.ID {
		t.Fatalf("unexpected list order: %+v", list)
	}

	// 3. Reorder: m3, m1, m2
	if err := repos.EmbyMount.SetSortOrder(ctx, []string{m3.ID, m1.ID, m2.ID}); err != nil {
		t.Fatalf("SetSortOrder failed: %v", err)
	}

	// 4. Query list again and verify updated order
	reordered, err := repos.EmbyMount.List(ctx)
	if err != nil {
		t.Fatalf("list mounts after reorder: %v", err)
	}
	if len(reordered) != 3 {
		t.Fatalf("expected 3 mounts, got %d", len(reordered))
	}
	if reordered[0].ID != m3.ID || reordered[1].ID != m1.ID || reordered[2].ID != m2.ID {
		t.Fatalf("expected order [m3, m1, m2], got: %s, %s, %s",
			reordered[0].ID, reordered[1].ID, reordered[2].ID)
	}
	if reordered[0].SortOrder != 0 || reordered[1].SortOrder != 1 || reordered[2].SortOrder != 2 {
		t.Fatalf("unexpected sort orders: %d, %d, %d",
			reordered[0].SortOrder, reordered[1].SortOrder, reordered[2].SortOrder)
	}
}
