// dbsetup 数据库建表/初始化脚本执行工具（开发联调用）
// 用法：go run ./cmd/dbsetup [sql 文件...]
// 默认执行 sql/ 目录下 001_schema、002_init_data、003_seed 三个脚本
// 说明：T-SQL 脚本中的 GO 分隔符由本工具拆分后逐批执行
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"hospital-qc-wework/internal/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/denisenkom/go-mssqldb" // SQL Server 驱动
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()
	scripts := flag.Args()
	if len(scripts) == 0 {
		scripts = []string{"../sql/001_schema.sql", "../sql/002_init_data.sql", "../sql/003_seed.sql"}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("连接数据库 %s/%s ...\n", cfg.Database.Server, cfg.Database.Name)
	db, err := sqlx.Connect("sqlserver", cfg.Database.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("连接成功")

	ok := true
	for _, path := range scripts {
		fmt.Printf("=== 执行 %s ===\n", path)
		if err := runScript(db, path); err != nil {
			fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", path, err)
			ok = false
			continue
		}
		fmt.Printf("[OK] %s\n", path)
	}
	if !ok {
		os.Exit(1)
	}
	fmt.Println("全部脚本执行完成")
}

// runScript 读取脚本并按 GO 拆分批次执行
func runScript(db *sqlx.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	batches := splitBatches(string(data))
	for i, b := range batches {
		if strings.TrimSpace(b) == "" {
			continue
		}
		if _, err := db.Exec(b); err != nil {
			return fmt.Errorf("批次 %d/%d 执行失败: %w\nSQL:\n%s", i+1, len(batches), err, b)
		}
		fmt.Printf("  批次 %d/%d OK\n", i+1, len(batches))
	}
	return nil
}

// splitBatches 按单独的 GO 行拆分 T-SQL 批次
func splitBatches(sql string) []string {
	var batches []string
	var cur strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		if strings.TrimSpace(strings.ToUpper(line)) == "GO" {
			batches = append(batches, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	if cur.Len() > 0 {
		batches = append(batches, cur.String())
	}
	return batches
}
