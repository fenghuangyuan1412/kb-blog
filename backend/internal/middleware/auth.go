// Package middleware 放与 http 相关的横切逻辑：身份识别、跨域。
package middleware

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/render"
)

type ctxKey string

const viewerEmailKey ctxKey = "viewer_email"

// TokenVerifier 是中间件对认证服务的最小要求。
type TokenVerifier interface {
	Verify(token string) (string, error)
}

// EmailFromContext 返回已登录管理员邮箱，未登录为空串。
func EmailFromContext(ctx context.Context) string {
	v, _ := ctx.Value(viewerEmailKey).(string)
	return v
}

// IsViewer 表示当前请求是否带有效令牌（能看到 draft）。
func IsViewer(ctx context.Context) bool {
	return EmailFromContext(ctx) != ""
}

// Optional 尝试从 Cookie 认人：认出来就写进 context，认不出来也放行。
// 公开读接口用它，管理员才能在自己站子上看到草稿。
func Optional(cookieName string, v TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
				if email, err := v.Verify(cookie.Value); err == nil {
					r = r.WithContext(context.WithValue(r.Context(), viewerEmailKey, email))
				} else if !isUnauthorized(err) {
					render.Error(w, http.StatusInternalServerError, "internal_error", "服务端内部错误")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Require 拦住所有没有有效令牌的请求，并写入邮箱供 handler 使用。
func Require(cookieName string, v TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				render.Error(w, http.StatusUnauthorized, "unauthorized", model.ErrUnauthorized.Error())
				return
			}
			email, err := v.Verify(cookie.Value)
			if err != nil {
				render.ErrorFromDomain(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), viewerEmailKey, email)))
		})
	}
}

// CORS 只放行白名单里的源，并允许带 Cookie。本机前端 5173 → 后端 8080 就靠它。
func CORS(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				h := w.Header()
				h.Set("Access-Control-Allow-Methods", strings.Join([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, ", "))
				h.Set("Access-Control-Allow-Headers", strings.Join([]string{"Content-Type", "Accept"}, ", "))
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isUnauthorized(err error) bool {
	return errors.Is(err, model.ErrUnauthorized)
}
