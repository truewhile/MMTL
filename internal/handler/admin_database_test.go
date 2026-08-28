package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/service"
)

func TestBuildDSN(t *testing.T) {
	cases := []struct {
		payload DatabaseConnectionPayload
		want    string
	}{
		{
			payload: DatabaseConnectionPayload{
				DSN: "postgres://myuser:mypass@10.0.0.1:5432/mydb?sslmode=require",
			},
			want: "postgres://myuser:mypass@10.0.0.1:5432/mydb?sslmode=require",
		},
		{
			payload: DatabaseConnectionPayload{
				Host:     "127.0.0.1",
				Port:     5432,
				User:     "postgres",
				Password: "secretpassword",
				DBName:   "mmtl_prod",
				SSLMode:  "disable",
			},
			want: "postgres://postgres:secretpassword@127.0.0.1:5432/mmtl_prod?sslmode=disable",
		},
	}

	for _, c := range cases {
		got := c.payload.BuildDSN()
		if got != c.want {
			t.Errorf("BuildDSN() = %q, want %q", got, c.want)
		}
	}
}

func TestGetDatabaseStatusHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	cfg.Database.DBPath = "./data/mmtl.db"

	svc := &service.Container{
		Database: service.NewDatabaseAdminService(cfg, nil, nil, nil),
	}

	r := gin.New()
	r.GET("/api/admin/database/status", getDatabaseStatusHandler(svc))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/database/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["type"] != "sqlite" {
		t.Fatalf("expected type=sqlite, got %v", resp["type"])
	}
}

func TestSaveDatabaseConfigHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.App.DataDir = dir
	cfg.Database.Type = "sqlite"

	svc := &service.Container{
		Database: service.NewDatabaseAdminService(cfg, nil, nil, nil),
	}

	r := gin.New()
	r.POST("/api/admin/database/save-config", saveDatabaseConfigHandler(svc))

	body := bytes.NewBufferString(`{"type":"postgres","host":"localhost","port":5432,"user":"admin","password":"pwd","dbname":"mmtl"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/database/save-config", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
