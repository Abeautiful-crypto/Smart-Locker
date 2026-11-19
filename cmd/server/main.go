// Smart-Locker 服务入口：REST API + 静态资源（三端 SPA）。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"smart-locker/internal/httpapi"
	"smart-locker/internal/store"
)

func main() {
	dsn := os.Getenv("LOCKER_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/smart_locker?charset=utf8mb4&parseTime=true&loc=Local"
	}
	addr := os.Getenv("LOCKER_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	var st store.Store
	if dsn == "mem" {
		log.Println("[locker] 使用内存存储（mem），数据不持久化")
		st = store.NewMem()
	} else {
		db, err := store.NewMySQL(dsn)
		if err != nil {
			log.Fatalf("[locker] MySQL 连接失败: %v", err)
		}
		st = db
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := db.Migrate(ctx, "migrations"); err != nil {
			log.Printf("[locker] 迁移执行警告: %v", err)
		}
		cancel()
		log.Println("[locker] MySQL 就绪，迁移已执行")
	}
	defer st.Close()

	srv := httpapi.New(st)
	mux := http.NewServeMux()
	srv.Routes(mux)

	// 静态资源（三端 SPA）
	fs := http.FileServer(http.Dir("web"))
	mux.Handle("GET /", fs)

	addr = strings.TrimSpace(addr)
	log.Printf("[locker] Smart-Locker 服务启动 http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
