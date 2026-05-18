package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"magicgateway/auth"
	"magicgateway/config"
	"magicgateway/handler"
	"magicgateway/proxy"
	"magicgateway/store"
)

var Version = "dev"

//go:embed web/*
var webFiles embed.FS

func main() {
	// Load config
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// Setup file logger
	if cfg.Server.LogFile != "" {
		f, err := os.OpenFile(cfg.Server.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Open log file: %v", err)
		}
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, f))
	}

	// Ensure data directory exists
	dataDir := filepath.Dir(cfg.Database.Path)
	if dataDir != "." {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Fatalf("Create data dir: %v", err)
		}
	}

	// Init database
	st, err := store.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer st.Close()

	// Create default admin
	if err := st.CreateDefaultAdmin(cfg.Admin.DefaultUsername, cfg.Admin.DefaultPassword); err != nil {
		log.Fatalf("Create admin: %v", err)
	}

	// Handlers
	authH := &handler.AuthHandler{Store: st, JWTSecret: cfg.Server.JWTSecret}
	keyH := &handler.KeyHandler{Store: st}
	statsH := &handler.StatsHandler{Store: st}
	adminH := &handler.AdminHandler{Store: st}

	// Proxy
	p := proxy.New(st, cfg.DeepSeek.BaseURL, cfg.DeepSeek.APIKey)

	// JWT middleware
	jwtMw := auth.JWTAuth(cfg.Server.JWTSecret)

	// Router (Go 1.22 enhanced mux)
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"` + Version + `"}`))
	})

	// Public endpoints
	mux.HandleFunc("POST /api/auth/login", authH.Login)
	mux.HandleFunc("POST /api/auth/register", authH.Register)

	// User endpoints (JWT required)
	mux.Handle("PUT /api/auth/password", jwtMw(http.HandlerFunc(authH.ChangePassword)))
	mux.Handle("GET /api/keys", jwtMw(http.HandlerFunc(keyH.ListMyKeys)))
	mux.Handle("POST /api/keys", jwtMw(http.HandlerFunc(keyH.CreateKey)))
	mux.Handle("DELETE /api/keys/{id}", jwtMw(http.HandlerFunc(keyH.RevokeKey)))
	mux.Handle("PUT /api/keys/{id}/enable", jwtMw(http.HandlerFunc(keyH.EnableKey)))
	mux.Handle("GET /api/stats/mine", jwtMw(http.HandlerFunc(statsH.MyStats)))
	mux.Handle("GET /api/stats/ranking", jwtMw(http.HandlerFunc(statsH.Ranking)))
	mux.Handle("GET /api/stats/breakdown", jwtMw(http.HandlerFunc(statsH.Breakdown)))

	// Admin endpoints (JWT + admin role required)
	mux.Handle("GET /api/admin/users", jwtMw(http.HandlerFunc(auth.AdminOnly(adminH.ListUsers))))
	mux.Handle("DELETE /api/admin/users/{id}", jwtMw(http.HandlerFunc(auth.AdminOnly(adminH.DeleteUser))))
	mux.Handle("PUT /api/admin/users/{id}/password", jwtMw(http.HandlerFunc(auth.AdminOnly(authH.AdminResetPassword))))
	mux.Handle("GET /api/admin/keys", jwtMw(http.HandlerFunc(auth.AdminOnly(adminH.ListAllKeys))))
	mux.Handle("POST /api/admin/keys", jwtMw(http.HandlerFunc(auth.AdminOnly(adminH.CreateKey))))
	mux.Handle("DELETE /api/admin/keys/{id}", jwtMw(http.HandlerFunc(auth.AdminOnly(adminH.RevokeKey))))
	mux.Handle("PUT /api/admin/keys/{id}/enable", jwtMw(http.HandlerFunc(auth.AdminOnly(adminH.EnableKey))))
	mux.Handle("GET /api/admin/users/stats", jwtMw(http.HandlerFunc(auth.AdminOnly(adminH.ListUsersWithStats))))
	mux.Handle("GET /api/admin/stats/all", jwtMw(http.HandlerFunc(auth.AdminOnly(statsH.AllStats))))
	mux.Handle("GET /api/admin/stats/ranking", jwtMw(http.HandlerFunc(auth.AdminOnly(statsH.Ranking))))
	mux.Handle("GET /api/admin/stats/overview", jwtMw(http.HandlerFunc(auth.AdminOnly(statsH.Overview))))

	// Proxy endpoint (API key auth, no JWT)
	mux.Handle("POST /v1/messages", p)

	// Web pages
	webFS, _ := fs.Sub(webFiles, "web")
	mux.HandleFunc("GET /login", serveFile(webFS, "login.html"))
	mux.HandleFunc("GET /dashboard", serveFile(webFS, "dashboard.html"))
	mux.HandleFunc("GET /admin", serveFile(webFS, "admin.html"))

	// Redirect root to login
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// Start
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("MagicGateway v%s starting on %s", Version, addr)
	log.Printf("Login page: http://localhost%s/login", addr)
	log.Printf("Admin user: %s", cfg.Admin.DefaultUsername)

	if err := http.ListenAndServe(addr, handler.RecoverMiddleware(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func serveFile(fsys fs.FS, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}
