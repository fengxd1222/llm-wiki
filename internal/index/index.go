package index

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"

	// SQLite 驱动（modernc.org/sqlite，纯 Go，W0 验证项 1 通过）。
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// sqliteDriverName 是 modernc.org/sqlite 注册的 database/sql 驱动名。
const sqliteDriverName = "sqlite"

// migrationsDir 是 embed.FS 内 migration 文件所在子目录。
const migrationsDir = "migrations"

// ErrIndexUnavailable 表示 SQLite 索引无法初始化（包含 migration 失败）。
var ErrIndexUnavailable = errors.New("index unavailable")

// DB 是 WikiMind SQLite 索引的封装，负责 .wikimind/index.db 的生命周期。
//
// 单 vault 单 daemon 单 writer 的物理边界由调用方（daemon / CLI）保证；
// 当前 D3 没有 daemon，CLI 路径直接 Open/Close 一次。
type DB struct {
	sql  *sql.DB
	path string
}

// Open 打开（或建立）vaultRoot/.wikimind/index.db 并跑 goose up 到最新版本。
//
// 失败时会清理已建立的连接。调用方负责在使用完毕后调用 Close。
func Open(vaultRoot string) (*DB, error) {
	if vaultRoot == "" {
		return nil, fmt.Errorf("%w: vault root is empty", ErrIndexUnavailable)
	}
	absRoot, err := filepath.Abs(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve vault root: %v", ErrIndexUnavailable, err)
	}
	dbDir := filepath.Join(absRoot, ".wikimind")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("%w: create .wikimind: %v", ErrIndexUnavailable, err)
	}
	dbPath := filepath.Join(dbDir, "index.db")

	// 启动前备份（engineering-decisions §3.4：migration 失败可用备份回滚）。
	if err := backupIfExists(dbPath); err != nil {
		return nil, fmt.Errorf("%w: backup index.db: %v", ErrIndexUnavailable, err)
	}

	sqlDB, err := sql.Open(sqliteDriverName, dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w: open sqlite: %v", ErrIndexUnavailable, err)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("%w: ping sqlite: %v", ErrIndexUnavailable, err)
	}
	if err := applyMigrations(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("%w: %v", ErrIndexUnavailable, err)
	}

	return &DB{sql: sqlDB, path: dbPath}, nil
}

// Close 关闭底层 *sql.DB。重复调用是安全的（返回首个非 nil 错误）。
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	err := d.sql.Close()
	d.sql = nil
	return err
}

// BeginTx 开启一个事务，封装底层 *sql.DB.BeginTx。
func (d *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if d == nil || d.sql == nil {
		return nil, ErrIndexUnavailable
	}
	return d.sql.BeginTx(ctx, opts)
}

// SQL 暴露底层 *sql.DB 供 service 层使用查询。
//
// 仍以串行写为目标（commit loop），SQL 直接给 service 是 D3 的简化路径；
// 引入 daemon 后会收敛到 commit 包。
func (d *DB) SQL() *sql.DB {
	if d == nil {
		return nil
	}
	return d.sql
}

// Path 返回 index.db 的绝对路径。
func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

// applyMigrations 使用嵌入的 migrations/*.sql 跑 goose up。
func applyMigrations(db *sql.DB) error {
	sub, err := newMigrationsProvider(db)
	if err != nil {
		return err
	}
	if _, err := sub.Up(context.Background()); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func newMigrationsProvider(db *sql.DB) (*goose.Provider, error) {
	sub, err := fs.Sub(migrationsFS, migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("sub migrations fs: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		db,
		sub,
	)
	if err != nil {
		return nil, fmt.Errorf("goose new provider: %w", err)
	}
	return provider, nil
}

// backupIfExists 在 daemon 启动前把现有 index.db 备份到 index.db.bak。
// 不存在视为正常（首次启动）。
func backupIfExists(dbPath string) error {
	src, err := os.Open(dbPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer src.Close()

	bak := dbPath + ".bak"
	dst, err := os.OpenFile(bak, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}
