package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/config"
	"github.com/truewhile/MeBox/internal/model"
)

func TestMaskDSN(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			in:   "postgres://admin:secret123@localhost:5432/mebox?sslmode=disable",
			want: "postgres://admin:******@localhost:5432/mebox?sslmode=disable",
		},
		{
			in:   "host=localhost port=5432 user=admin password=secret dbname=mebox sslmode=disable",
			want: "host=localhost port=5432 user=admin password=****** dbname=mebox sslmode=disable",
		},
		{
			in:   "sqlite://data/mebox.db",
			want: "sqlite://data/mebox.db",
		},
		{
			in:   "",
			want: "",
		},
	}

	for _, c := range cases {
		got := MaskDSN(c.in)
		if got != c.want {
			t.Errorf("MaskDSN(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInspectDatabaseStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Media{}); err != nil {
		t.Fatal(err)
	}
	_ = db.Create(&model.User{Username: "testuser", PasswordHash: "h", Role: "user"}).Error

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	cfg.Database.DBPath = "./data/mebox.db"

	st := InspectDatabaseStatus(db, cfg)
	if st == nil {
		t.Fatal("expected non-nil DatabaseStatus")
	}
	if st.Type != "sqlite" {
		t.Fatalf("expected sqlite, got %s", st.Type)
	}
	if st.DBPath != "./data/mebox.db" {
		t.Fatalf("expected db_path, got %s", st.DBPath)
	}
	if st.TableCounts["users"] != 1 {
		t.Fatalf("expected 1 user, got %d", st.TableCounts["users"])
	}
}
