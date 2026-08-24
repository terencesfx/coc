package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mushan/coc/internal/account"
	"github.com/mushan/coc/internal/config"
	"github.com/mushan/coc/internal/database"
	"github.com/mushan/coc/internal/maintenance"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "create":
		createAdmin()
	case "restore":
		restoreBackup()
	default:
		usage()
		os.Exit(2)
	}
}

func createAdmin() {
	flags := flag.NewFlagSet("create", flag.ExitOnError)
	username := flags.String("username", "admin", "管理员用户名")
	displayName := flags.String("display-name", "管理员", "管理员显示名称")
	_ = flags.Parse(os.Args[2:])
	password := os.Getenv("COC_ADMIN_PASSWORD")
	if password == "" {
		fmt.Fprintln(os.Stderr, "COC_ADMIN_PASSWORD 不能为空")
		os.Exit(2)
	}

	cfg := config.Load()
	db, err := database.Open(context.Background(), cfg.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	created, err := account.NewStore(db).Create(
		context.Background(), *username, *displayName, password, "admin", false,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建管理员失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("管理员 %s (%s) 已创建\n", created.DisplayName, created.Username)
}

func restoreBackup() {
	flags := flag.NewFlagSet("restore", flag.ExitOnError)
	bundle := flags.String("bundle", "", "完整备份包路径")
	confirm := flags.String("confirm", "", "必须填写 RESTORE")
	_ = flags.Parse(os.Args[2:])
	if *bundle == "" || *confirm != "RESTORE" {
		fmt.Fprintln(os.Stderr, "恢复要求 -bundle 和 -confirm RESTORE；执行前必须停止 API 服务")
		os.Exit(2)
	}
	bundlePath, err := filepath.Abs(*bundle)
	if err != nil {
		fail("解析备份路径", err)
	}
	cfg := config.Load()
	dataLock, err := maintenance.AcquireDataLock(cfg.DatabasePath)
	if err != nil {
		fail("获取数据锁（请先停止 API 服务）", err)
	}
	defer dataLock.Close()
	validator := maintenance.New(nil, cfg.DatabasePath, cfg.BackupDir, time.Now().UTC(), cfg.AssetDir, cfg.CustomOccupationsPath)
	if _, err := validator.ValidateBundle(bundlePath); err != nil {
		fail("备份包校验失败，当前数据未修改", err)
	}
	db, err := database.Open(context.Background(), cfg.DatabasePath)
	if err != nil {
		fail("打开当前数据库", err)
	}
	safetyService := maintenance.New(db, cfg.DatabasePath, cfg.BackupDir, time.Now().UTC(), cfg.AssetDir, cfg.CustomOccupationsPath)
	safety, err := safetyService.CreateBackup(context.Background())
	if err != nil {
		_ = db.Close()
		fail("创建恢复前安全备份", err)
	}
	if err := db.Close(); err != nil {
		fail("关闭当前数据库", err)
	}
	result, err := validator.RestoreBundle(bundlePath)
	if err != nil {
		fail("恢复失败；请保留安全备份 "+safety.Name, err)
	}
	fmt.Printf("恢复完成：数据库 %s，图片 %d 个\n恢复前安全备份：%s\n现在可以重新启动 API 服务。\n", result.DatabasePath, result.AssetCount, safety.Name)
}

func fail(operation string, err error) {
	fmt.Fprintf(os.Stderr, "%s失败: %v\n", operation, err)
	os.Exit(1)
}
func usage() {
	fmt.Fprintln(os.Stderr, "用法:\n  COC_ADMIN_PASSWORD=... go run ./cmd/admin create -username admin -display-name 管理员\n  go run ./cmd/admin restore -bundle /path/to/coc-backup.tar.gz -confirm RESTORE")
}
