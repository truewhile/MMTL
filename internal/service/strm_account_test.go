package service

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"

	"github.com/truewhile/MeBox/internal/config"
	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
)

func TestStrmAccountConfigPreviewOf(t *testing.T) {
	svc := testStrmService(t)
	acct := &model.StrmAccount{
		Provider: model.StrmProviderOpenList,
		Config: mustJSON(map[string]string{
			"server":   "https://list.example.com",
			"username": "alice",
			"password": svc.crypto.Encrypt("secret"),
			"token":    svc.crypto.Encrypt("tok"),
		}),
	}

	preview := svc.StrmAccountConfigPreviewOf(acct)
	if preview.Server != "https://list.example.com" {
		t.Fatalf("server = %q", preview.Server)
	}
	if preview.Username != "alice" {
		t.Fatalf("username = %q", preview.Username)
	}
	if !preview.HasPassword || !preview.HasToken {
		t.Fatalf("expected secret flags, got %#v", preview)
	}
}

func TestUpdateStrmAccountMergesConfigWithoutClearingSecrets(t *testing.T) {
	svc := testStrmService(t)
	ctx := context.Background()
	acct, err := svc.CreateStrmAccount(ctx, "openlist", model.StrmProviderOpenList, map[string]string{
		"server":   "https://list.example.com",
		"username": "alice",
		"password": "secret",
		"token":    "tok",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	updated, err := svc.UpdateStrmAccount(ctx, acct.ID, "openlist-renamed", nil, map[string]string{
		"server":   "https://list.example.com",
		"username": "alice",
	})
	if err != nil {
		t.Fatalf("update account: %v", err)
	}
	if updated.Name != "openlist-renamed" {
		t.Fatalf("name = %q", updated.Name)
	}
	cfg, err := svc.strmAccountConfig(updated)
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg["password"] != "secret" {
		t.Fatalf("password was cleared: %#v", cfg)
	}
	if cfg["token"] != "tok" {
		t.Fatalf("token was cleared: %#v", cfg)
	}
}

func TestDeleteStrmAccountCascadesEmbyMounts(t *testing.T) {
	ctx := context.Background()
	db := newServiceTestDB(t, &model.StrmAccount{}, &model.EmbyMount{}, &model.StrmSyncPath{})
	repos := repository.New(db)
	svc := NewStrmService(nil, zap.NewNop(), repos, nil)

	createEmbyAcct := func(id, name string) *model.StrmAccount {
		acct := &model.StrmAccount{
			Base:     model.Base{ID: id},
			Name:     name,
			Provider: model.StrmProviderEmbyRemote,
			Enabled:  true,
		}
		if err := repos.StrmAccount.Create(ctx, acct); err != nil {
			t.Fatalf("create account %s: %v", id, err)
		}
		return acct
	}
	createMounts := func(accountID string, viewIDs ...string) {
		for _, vid := range viewIDs {
			m := &model.EmbyMount{
				AccountID:      accountID,
				RemoteViewID:   vid,
				RemoteViewName: "库-" + vid,
				Enabled:        true,
			}
			if err := repos.EmbyMount.Create(ctx, m); err != nil {
				t.Fatalf("create mount %s: %v", vid, err)
			}
		}
	}

	gone := createEmbyAcct("acct-gone", "要删除的账号")
	createMounts(gone.ID, "view-1", "view-2", "view-3")
	keep := createEmbyAcct("acct-keep", "保留的账号")
	createMounts(keep.ID, "view-a")

	if err := svc.DeleteStrmAccount(ctx, gone.ID); err != nil {
		t.Fatalf("DeleteStrmAccount failed: %v", err)
	}

	remaining, err := repos.StrmAccount.FindByID(ctx, gone.ID)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if remaining != nil {
		t.Fatalf("account %s should be deleted", gone.ID)
	}
	goneMounts, err := repos.EmbyMount.ListByAccountID(ctx, gone.ID)
	if err != nil {
		t.Fatalf("list mounts: %v", err)
	}
	if len(goneMounts) != 0 {
		t.Fatalf("deleted account still has %d mounts (orphans)", len(goneMounts))
	}
	keepMounts, err := repos.EmbyMount.ListByAccountID(ctx, keep.ID)
	if err != nil {
		t.Fatalf("list mounts: %v", err)
	}
	if len(keepMounts) != 1 || keepMounts[0].RemoteViewID != "view-a" {
		t.Fatalf("keep account mounts = %#v, want 1 (view-a)", keepMounts)
	}
}

func TestCleanupOrphanMountsRemovesStaleRows(t *testing.T) {
	ctx := context.Background()
	db := newServiceTestDB(t, &model.StrmAccount{}, &model.EmbyMount{})
	repos := repository.New(db)
	svc := NewEmbyRemoteService(&config.Config{}, zap.NewNop(), repos, nil)

	acct := &model.StrmAccount{
		Base:     model.Base{ID: "acct-1"},
		Name:     "emby",
		Provider: model.StrmProviderEmbyRemote,
		Enabled:  true,
	}
	if err := repos.StrmAccount.Create(ctx, acct); err != nil {
		t.Fatalf("create account: %v", err)
	}
	for i, vid := range []string{"v1", "v2", "v-orphan-1", "v-orphan-2"} {
		m := &model.EmbyMount{
			Base:         model.Base{ID: "mount-" + vid},
			AccountID:    acct.ID,
			RemoteViewID: vid,
			Enabled:      true,
		}
		if i >= 2 {
			// 模拟历史残留：挂载归属不存在的账号
			m.AccountID = "no-such-account"
		}
		if err := repos.EmbyMount.Create(ctx, m); err != nil {
			t.Fatalf("create mount %s: %v", vid, err)
		}
	}

	svc.CleanupOrphanMounts(ctx)

	left, err := repos.EmbyMount.List(ctx)
	if err != nil {
		t.Fatalf("list mounts: %v", err)
	}
	if len(left) != 2 {
		t.Fatalf("mounts after cleanup = %d, want 2 (orphans removed)", len(left))
	}
	for _, m := range left {
		if m.AccountID != acct.ID {
			t.Fatalf("mount %s still orphan (account %s)", m.ID, m.AccountID)
		}
	}
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
