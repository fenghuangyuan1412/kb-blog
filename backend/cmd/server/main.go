// kb-blog 后端入口：读配置、连数据库、装配分层、起 HTTP 服务。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/config"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/db"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/handler"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/middleware"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/render"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/repo"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/service"
)

const version = "0.1.0"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := run(); err != nil {
		slog.Error("启动失败", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	documentRepo := repo.New(pool)
	documentSvc := service.NewDocument(documentRepo)
	authSvc := service.NewAuthService(cfg)
	authH := handler.NewAuth(authSvc, cfg.CookieName, cfg.CookieSecure)
	docH := handler.NewDocument(documentSvc)

	optional := middleware.Optional(cfg.CookieName, authSvc)
	required := middleware.Require(cfg.CookieName, authSvc)

	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Logger, chimw.Recoverer)
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Get("/healthz", healthz(pool))

	r.Route("/api", func(r chi.Router) {
		// 公开读：挂 optional 中间件，管理员能看到自己的草稿
		r.With(optional).Get("/documents", docH.List)
		r.With(optional).Get("/documents/{slug}", docH.GetBySlug)
		r.With(optional).Get("/notes/tree", docH.NotesTree)
		r.With(optional).Get("/tags", docH.Tags)
		r.With(optional).Get("/search", docH.Search)

		r.Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)
		r.With(required).Get("/auth/me", authH.Me)

		r.With(required).Post("/documents", docH.Create)
		r.With(required).Put("/documents/{id}", docH.Update)
		r.With(required).Delete("/documents/{id}", docH.Delete)
		r.With(required).Post("/documents/{id}/move", docH.Move)
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		slog.Info("收到退出信号，正在关闭")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("关闭服务超时", "err", err)
		}
	}()

	slog.Info("服务已启动", "addr", cfg.Addr, "version", version)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	}
	<-shutdownDone
	return nil
}

// healthz 顺带探一次数据库，部署平台可以直接用它做存活检查。
func healthz(pool *pgxpool.Pool) http.HandlerFunc {
	type okBody struct {
		Status  string `json:"status"`
		DB      string `json:"db"`
		Version string `json:"version"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			render.Error(w, http.StatusServiceUnavailable, "db_unavailable", "数据库不可用")
			return
		}
		render.JSON(w, http.StatusOK, okBody{Status: "ok", DB: "ok", Version: version})
	}
}
