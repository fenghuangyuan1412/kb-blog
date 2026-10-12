package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
)

// stubVerifier 代替真实认证服务，把注意力放在中间件本身的行为上。
type stubVerifier struct {
	email string
	err   error
}

func (s stubVerifier) Verify(string) (string, error) { return s.email, s.err }

const cookieName = "kb_token"

func okHandlerEmail() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Viewer", EmailFromContext(r.Context()))
		w.WriteHeader(http.StatusOK)
	})
}

func doWithCookie(mw func(http.Handler) http.Handler, value string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/documents", nil)
	if value != "" {
		r.AddCookie(&http.Cookie{Name: cookieName, Value: value})
	}
	rec := httptest.NewRecorder()
	mw(okHandlerEmail()).ServeHTTP(rec, r)
	return rec
}

func TestOptional(t *testing.T) {
	cases := []struct {
		name       string
		verifier   TokenVerifier
		cookieVal  string
		wantStatus int
		wantViewer string
	}{
		{"无 Cookie 匿名放行", stubVerifier{}, "", http.StatusOK, ""},
		{"有效令牌认出管理员", stubVerifier{email: "admin@kb.local"}, "ok", http.StatusOK, "admin@kb.local"},
		{"坏令牌降级成匿名", stubVerifier{err: model.ErrUnauthorized}, "bad", http.StatusOK, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := doWithCookie(Optional(cookieName, c.verifier), c.cookieVal)
			if rec.Code != c.wantStatus {
				t.Fatalf("状态码 = %d, 期望 %d", rec.Code, c.wantStatus)
			}
			if got := rec.Header().Get("X-Viewer"); got != c.wantViewer {
				t.Fatalf("context 里的邮箱 = %q, 期望 %q", got, c.wantViewer)
			}
		})
	}

	t.Run("验证器本身出错要报 500 且不进 handler", func(t *testing.T) {
		v := stubVerifier{err: errors.New("数据库挂了")}
		rec := doWithCookie(Optional(cookieName, v), "whatever")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("状态码 = %d, 期望 500", rec.Code)
		}
		if rec.Header().Get("X-Viewer") != "" {
			t.Fatal("500 之后不该继续走 handler")
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("响应不是统一错误格式: %v (%s)", err, rec.Body.String())
		}
		if body.Error.Code != "internal_error" {
			t.Fatalf("错误码 = %q", body.Error.Code)
		}
	})
}

func TestRequire(t *testing.T) {
	t.Run("有效令牌放行并写入邮箱", func(t *testing.T) {
		v := stubVerifier{email: "admin@kb.local"}
		rec := doWithCookie(Require(cookieName, v), "ok")
		if rec.Code != http.StatusOK {
			t.Fatalf("状态码 = %d body=%s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Viewer") != "admin@kb.local" {
			t.Fatalf("邮箱没写进 context: %q", rec.Header().Get("X-Viewer"))
		}
	})

	cases := []struct {
		name      string
		verifier  TokenVerifier
		cookieVal string
		wantCode  string
	}{
		{"没带 Cookie", stubVerifier{email: "admin@kb.local"}, "", "unauthorized"},
		{"令牌无效", stubVerifier{err: model.ErrUnauthorized}, "junk", "unauthorized"},
		{"凭据不对", stubVerifier{err: model.ErrWrongCredential}, "junk", "wrong_credentials"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := doWithCookie(Require(cookieName, c.verifier), c.cookieVal)
			if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
				t.Fatalf("状态码 = %d, 期望 401/403", rec.Code)
			}
			if rec.Header().Get("X-Viewer") != "" {
				t.Fatal("被拦住却执行了 handler")
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("解析响应失败: %v (%s)", err, rec.Body.String())
			}
			if body.Error.Code != c.wantCode {
				t.Fatalf("错误码 = %q, 期望 %q", body.Error.Code, c.wantCode)
			}
		})
	}
}

func TestEmailFromContext(t *testing.T) {
	if EmailFromContext(context.Background()) != "" {
		t.Fatal("空 context 不该有邮箱")
	}
	if IsViewer(context.Background()) {
		t.Fatal("空 context 不该算已登录")
	}
	ctx := context.WithValue(context.Background(), viewerEmailKey, "admin@kb.local")
	if !IsViewer(ctx) {
		t.Fatal("写入邮箱后应算管理员")
	}
}

func TestCORS(t *testing.T) {
	h := CORS([]string{"http://localhost:5173"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("白名单内的源", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/documents", nil)
		r.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Fatal("没回显允许的源")
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Fatal("没允许带 Cookie，前端登录态就废了")
		}
		if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
			t.Fatal("缺 Vary: Origin，缓存会串源")
		}
	})

	t.Run("白名单外的源", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/documents", nil)
		r.Header.Set("Origin", "http://evil.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("不该放行陌生源")
		}
	})

	t.Run("预检请求直接返回 204 不进 handler", func(t *testing.T) {
		called := false
		mw := CORS([]string{"http://localhost:5173"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}))
		r := httptest.NewRequest(http.MethodOptions, "/documents", nil)
		r.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, r)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("预检状态码 = %d", rec.Code)
		}
		if called {
			t.Fatal("预检不该打到业务 handler")
		}
		if rec.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Fatal("预检没告诉浏览器允许的方法")
		}
	})
}

// 确认错误响应体不会把内部信息漏出去。
func TestOptional_InternalErrorLeaksNothing(t *testing.T) {
	v := stubVerifier{err: errors.New("pq: 表不存在 密码=hunter2")}
	rec := doWithCookie(Optional(cookieName, v), "x")
	body, _ := io.ReadAll(rec.Body)
	if strings.Contains(string(body), "hunter2") || strings.Contains(string(body), "pq:") {
		t.Fatalf("500 响应泄漏了内部信息: %s", body)
	}
}
