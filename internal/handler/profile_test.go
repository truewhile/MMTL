package handler

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
	"github.com/truewhile/MeBox/internal/service"
)

func TestProfileHideAdultRequiresPasswordOnlyWhenChanged(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	user := &model.User{Username: "viewer", PasswordHash: "hash", Role: "user", HideAdult: true}
	if err := repos.User.Create(t.Context(), user); err != nil {
		t.Fatal(err)
	}
	svc := &service.Container{Repo: repos}

	same := true
	changed, err := profileHideAdultChanged(t.Context(), svc, user.ID, service.ProfileUpdate{HideAdult: &same})
	if err != nil {
		t.Fatalf("same value returned error: %v", err)
	}
	if changed {
		t.Fatal("same hide_adult value should not require password")
	}

	next := false
	changed, err = profileHideAdultChanged(t.Context(), svc, user.ID, service.ProfileUpdate{HideAdult: &next})
	if err != nil {
		t.Fatalf("changed value returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed hide_adult value should require password")
	}
}

func TestProfileUsernameChangeRequiresPasswordOnlyWhenChanged(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	user := &model.User{Username: "viewer", PasswordHash: "hash", Role: "user", HideAdult: true}
	if err := repos.User.Create(t.Context(), user); err != nil {
		t.Fatal(err)
	}
	svc := &service.Container{Repo: repos}

	same := " viewer "
	changed, err := profileUsernameChanged(t.Context(), svc, user.ID, service.ProfileUpdate{Username: &same})
	if err != nil {
		t.Fatalf("same username returned error: %v", err)
	}
	if changed {
		t.Fatal("same username after trimming should not require password")
	}

	next := "renamed"
	changed, err = profileUsernameChanged(t.Context(), svc, user.ID, service.ProfileUpdate{Username: &next})
	if err != nil {
		t.Fatalf("changed username returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed username should require password")
	}
}
