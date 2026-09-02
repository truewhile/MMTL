package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/truewhile/MeBox/internal/model"
)

func TestLoadMaxUsersDefaultsToTwenty(t *testing.T) {
	ctx := context.Background()
	repos, _, _, _ := newAuthTestServices(t)
	got, err := LoadMaxUsers(ctx, repos)
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultMaxUsers {
		t.Fatalf("max users = %d, want %d", got, DefaultMaxUsers)
	}
}

func TestSaveAndLoadMaxUsers(t *testing.T) {
	ctx := context.Background()
	repos, _, _, _ := newAuthTestServices(t)
	if err := SaveMaxUsers(ctx, repos, 50); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMaxUsers(ctx, repos)
	if err != nil {
		t.Fatal(err)
	}
	if got != 50 {
		t.Fatalf("max users = %d, want 50", got)
	}
}

func TestValidateMaxUsersRejectsInvalid(t *testing.T) {
	if err := ValidateMaxUsers(0); err == nil {
		t.Fatal("expected error for 0")
	}
	if err := ValidateMaxUsers(MaxUsersHardCap + 1); err == nil {
		t.Fatal("expected error above hard cap")
	}
}

func TestRegisterRespectsConfiguredUserLimit(t *testing.T) {
	ctx := context.Background()
	repos, auth, _, _ := newAuthTestServices(t)
	if err := SaveMaxUsers(ctx, repos, 3); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := repos.User.Create(ctx, &model.User{
			Username:     fmt.Sprintf("user-%02d", i),
			PasswordHash: "hash",
			Role:         "user",
			Tier:         "free",
		}); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err := auth.Register(ctx, "overflow", "password")
	if err == nil {
		t.Fatal("expected user limit error")
	}
}
