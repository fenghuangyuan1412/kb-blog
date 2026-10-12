// migrate 用 goose 应用 / 回滚 backend/migrations 下的 SQL。
// 用法：go run ./cmd/migrate [up|down|status]
// 表结构只能通过这里的迁移改，禁止手改数据库（见 CLAUDE.md 硬性约束）。
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/jackc/pgx/v5/stdlib" // 注册 database/sql 的 "pgx" 驱动
	"github.com/pressly/goose/v3"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/config"
)

func main() {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	dsn, err := config.DatabaseURL()
	if err != nil {
		exit(err)
	}

	dir, err := migrationsDir()
	if err != nil {
		exit(err)
	}

	// stdlib 在 init 里把驱动注册成 "pgx"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		exit(fmt.Errorf("打开数据库失败: %w", err))
	}
	defer db.Close()

	goose.SetDialect("postgres")

	switch cmd {
	case "up":
		err = goose.Up(db, dir)
	case "down":
		err = goose.Down(db, dir)
	case "status":
		err = goose.Status(db, dir)
	default:
		exit(fmt.Errorf("未知命令: %s（支持 up|down|status）", cmd))
	}
	if err != nil {
		exit(fmt.Errorf("执行 %s 失败: %w", cmd, err))
	}
}

// migrationsDir 兼容两种调用位置：仓库根（./backend/migrations）和 backend/（./migrations）。
func migrationsDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前目录失败: %w", err)
	}
	for i := 0; i < 5; i++ {
		for _, rel := range []string{"backend/migrations", "migrations"} {
			candidate := filepath.Join(wd, filepath.FromSlash(rel))
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				return candidate, nil
			}
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}
	return "", fmt.Errorf("没找到 migrations 目录")
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}
