package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ShukeBta/MMTL/internal/service"
)

type DatabaseConnectionPayload struct {
	Type     string `json:"type"`
	DSN      string `json:"dsn"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

func (p *DatabaseConnectionPayload) BuildDSN() string {
	raw := strings.TrimSpace(p.DSN)
	if raw != "" {
		return raw
	}
	host := strings.TrimSpace(p.Host)
	if host == "" {
		return ""
	}
	port := p.Port
	if port <= 0 {
		port = 5432
	}
	user := strings.TrimSpace(p.User)
	dbname := strings.TrimSpace(p.DBName)
	if dbname == "" {
		dbname = "mmtl"
	}
	sslmode := strings.TrimSpace(p.SSLMode)
	if sslmode == "" {
		sslmode = "disable"
	}

	userInfo := url.User(user)
	if p.Password != "" {
		userInfo = url.UserPassword(user, p.Password)
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     userInfo,
		Host:     fmt.Sprintf("%s:%d", host, port),
		Path:     "/" + dbname,
		RawQuery: "sslmode=" + url.QueryEscape(sslmode),
	}
	return u.String()
}

func getDatabaseStatusHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc.Database == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database service unavailable"})
			return
		}
		status := svc.Database.GetStatus(c.Request.Context())
		c.JSON(http.StatusOK, status)
	}
}

func testDatabaseHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DatabaseConnectionPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
			return
		}
		dsn := req.BuildDSN()
		if dsn == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 PostgreSQL 连接信息或 DSN"})
			return
		}
		res, err := svc.Database.TestPostgres(c.Request.Context(), dsn)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

func migrateDatabaseHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DatabaseConnectionPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
			return
		}
		dsn := req.BuildDSN()
		if dsn == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供目标 PostgreSQL 连接信息或 DSN"})
			return
		}
		res, err := svc.Database.MigrateToPostgres(c.Request.Context(), dsn)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "迁移失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

func saveDatabaseConfigHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DatabaseConnectionPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
			return
		}
		dbType := strings.ToLower(strings.TrimSpace(req.Type))
		if dbType == "" {
			dbType = "postgres"
		}
		var dsn string
		if dbType == "postgres" {
			dsn = req.BuildDSN()
			if dsn == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 PostgreSQL 连接信息或 DSN"})
				return
			}
		}

		if err := svc.Database.SaveConfig(c.Request.Context(), dbType, dsn); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "数据库配置已成功保存至配置文件，重启服务后将以新数据库运行",
			"type":    dbType,
		})
	}
}
