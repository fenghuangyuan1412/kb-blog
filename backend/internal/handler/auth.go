package handler

import (
	"net/http"
	"time"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/middleware"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/render"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/service"
)

type Auth struct {
	svc          *service.AuthService
	cookieName   string
	cookieSecure bool
}

func NewAuth(svc *service.AuthService, cookieName string, secure bool) *Auth {
	return &Auth{svc: svc, cookieName: cookieName, cookieSecure: secure}
}

type emailResponse struct {
	Email string `json:"email"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

// Login 校验凭据，成功后把 JWT 写进 httpOnly Cookie，前端读不到。
func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	token, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(service.TokenTTL / time.Second),
	})
	render.JSON(w, http.StatusOK, emailResponse{Email: h.svc.AdminEmail()})
}

// Logout 下发一个同名空 Cookie 把有效期清零。
func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	render.JSON(w, http.StatusOK, okResponse{OK: true})
}

// Me 返回当前登录身份，路由已挂 Require 中间件，这里只兜底。
func (h *Auth) Me(w http.ResponseWriter, r *http.Request) {
	email := middleware.EmailFromContext(r.Context())
	if email == "" {
		render.Error(w, http.StatusUnauthorized, "unauthorized", model.ErrUnauthorized.Error())
		return
	}
	render.JSON(w, http.StatusOK, emailResponse{Email: email})
}
