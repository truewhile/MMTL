package database

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"github.com/ShukeBta/MMTL/internal/config"
)

func installSQLiteWriteGate(db *gorm.DB) {
	if db == nil {
		return
	}
	const lockedKey = "mmtl:sqlite_write_locked"
	gate := newSQLiteWriteGate()
	lock := func(tx *gorm.DB) {
		ctx := context.Background()
		if tx.Statement != nil && tx.Statement.Context != nil {
			ctx = tx.Statement.Context
		}
		if err := gate.Lock(ctx); err != nil {
			_ = tx.AddError(err)
			return
		}
		tx.InstanceSet(lockedKey, struct{}{})
	}
	unlock := func(tx *gorm.DB) {
		if _, ok := tx.InstanceGet(lockedKey); ok {
			gate.Unlock()
		}
	}
	rawLock := func(tx *gorm.DB) {
		if tx.Statement != nil && isReadOnlySQL(tx.Statement.SQL.String()) {
			return
		}
		lock(tx)
	}
	_ = db.Callback().Create().Before("gorm:create").Register("mmtl:sqlite_write_lock", lock)
	_ = db.Callback().Create().After("gorm:create").Register("mmtl:sqlite_write_unlock", unlock)
	_ = db.Callback().Update().Before("gorm:update").Register("mmtl:sqlite_write_lock", lock)
	_ = db.Callback().Update().After("gorm:update").Register("mmtl:sqlite_write_unlock", unlock)
	_ = db.Callback().Delete().Before("gorm:delete").Register("mmtl:sqlite_write_lock", lock)
	_ = db.Callback().Delete().After("gorm:delete").Register("mmtl:sqlite_write_unlock", unlock)
	_ = db.Callback().Raw().Before("gorm:raw").Register("mmtl:sqlite_write_lock", rawLock)
	_ = db.Callback().Raw().After("gorm:raw").Register("mmtl:sqlite_write_unlock", unlock)
}

func isReadOnlySQL(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	if len(trimmed) == 0 {
		return false
	}
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "EXPLAIN") {
		return true
	}
	if strings.HasPrefix(upper, "WITH") && !strings.Contains(upper, "INSERT") && !strings.Contains(upper, "UPDATE") && !strings.Contains(upper, "DELETE") {
		return true
	}
	return false
}

// sqliteWriteGate serializes in-process SQLite writes while respecting the
// statement context, so request cancellation can break out of a queued write.
type sqliteWriteGate struct {
	ch chan struct{}
}

func newSQLiteWriteGate() *sqliteWriteGate {
	return &sqliteWriteGate{ch: make(chan struct{}, 1)}
}

func (g *sqliteWriteGate) Lock(ctx context.Context) error {
	select {
	case g.ch <- struct{}{}:
		return nil
	default:
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case g.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *sqliteWriteGate) Unlock() {
	select {
	case <-g.ch:
	default:
	}
}

func buildSQLiteDSN(cfg *config.Config) string {
	dbPath := cfg.Database.DBPath
	if !filepath.IsAbs(dbPath) {
		// keep as-is to respect user-provided relative paths.
		dbPath = filepath.Clean(dbPath)
	}
	dsn := dbPath + "?_pragma=foreign_keys(1)"
	if cfg.Database.WALMode {
		dsn += "&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	}
	if cfg.Database.BusyTimeout > 0 {
		dsn += fmt.Sprintf("&_pragma=busy_timeout(%d)", cfg.Database.BusyTimeout)
	}
	if cfg.Database.CacheSize != 0 {
		dsn += fmt.Sprintf("&_pragma=cache_size(%d)", cfg.Database.CacheSize)
	}
	dsn += "&_pragma=temp_store(MEMORY)&_pragma=mmap_size(536870912)"
	if cfg.Database.WALMode {
		dsn += "&_pragma=wal_autocheckpoint(1000)"
	}
	return dsn
}

func isSQLite(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite"
}

func isPostgres(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres"
}
