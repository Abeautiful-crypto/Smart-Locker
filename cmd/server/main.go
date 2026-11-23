// Smart-Locker 服务入口：REST API + 三端独立端口 SPA。
//
// 端口分配（可用同名环境变量覆盖）：
//
//	LOCKER_ADDR        :8090  兼容总入口（三端均可访问，e2e/冒烟脚本使用）
//	LOCKER_USER_ADDR   :8081  用户小程序端（u-*）
//	LOCKER_KIOSK_ADDR  :8082  柜机触屏端（k-*）
//	LOCKER_ADMIN_ADDR  :8083  管理后台（m-*）
//
// 每个端口共享同一套 REST API 与静态资源，仅入口 index.html 注入的
// window.LOCKER_END 不同，前端据此锁定本端首页与容器布局。
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

type endServer struct {
	addr  string
	end   string // u / k / m / all
	label string
}

func envAddr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// endHandler 构造单个端口的 handler：注册全部 API + 静态资源，
// 并把入口 HTML 的 __LOCKER_END__ 占位符替换为该端标识。
func endHandler(srv *httpapi.Server, end, indexHTML string) http.Handler {
	mux := http.NewServeMux()
	srv.Routes(mux)
	fs := http.FileServer(http.Dir("web"))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// 静态资源禁用缓存：避免浏览器拿 304 旧 JS/CSS 导致"改了没生效"
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(strings.ReplaceAll(indexHTML, "__LOCKER_END__", end)))
			return
		}
		fs.ServeHTTP(w, r)
	})
	return mux
}

func main() {
	dsn := os.Getenv("LOCKER_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/smart_locker?charset=utf8mb4&parseTime=true&loc=Local"
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

	indexBytes, err := os.ReadFile("web/index.html")
	if err != nil {
		log.Fatalf("[locker] 读取 web/index.html 失败: %v", err)
	}
	// 资源 URL 加版本戳：即使中间代理缓存了旧文件，版本号变化也会强制重新拉取
	indexHTML := strings.NewReplacer(
		"proto.css", "proto.css?v=20261123e",
		"app.css", "app.css?v=20261123e",
		"app.js", "app.js?v=20261123e",
	).Replace(string(indexBytes))

	srv := httpapi.New(st)
	ends := []endServer{
		{envAddr("LOCKER_ADDR", ":8090"), "all", "兼容总入口（三端）"},
		{envAddr("LOCKER_USER_ADDR", ":8081"), "u", "用户小程序端"},
		{envAddr("LOCKER_KIOSK_ADDR", ":8082"), "k", "柜机触屏端"},
		{envAddr("LOCKER_ADMIN_ADDR", ":8083"), "m", "管理后台"},
	}

	// 最后一个端口在主 goroutine 监听，其余在独立 goroutine，任一失败即退出
	for _, e := range ends[:len(ends)-1] {
		h := endHandler(srv, e.end, indexHTML)
		go func(e endServer, h http.Handler) {
			log.Printf("[locker] %s 启动 http://localhost%s (end=%s)", e.label, e.addr, e.end)
			if err := http.ListenAndServe(e.addr, h); err != nil {
				log.Fatalf("[locker] %s 监听失败: %v", e.label, err)
			}
		}(e, h)
	}
	last := ends[len(ends)-1]
	h := endHandler(srv, last.end, indexHTML)
	log.Printf("[locker] %s 启动 http://localhost%s (end=%s)", last.label, last.addr, last.end)
	if err := http.ListenAndServe(last.addr, h); err != nil {
		log.Fatalf("[locker] %s 监听失败: %v", last.label, err)
	}
}
