package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/truewhile/MeBox/internal/model"
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

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
