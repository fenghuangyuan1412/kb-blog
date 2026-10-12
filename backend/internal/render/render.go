// Package render 统一 HTTP 响应的形状：成功直接返回数据体，失败返回 { error: { code, message } }。
package render

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
)

type errorBody struct {
	Error errorObject `json:"error"`
}

type errorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON 写出成功响应。
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("序列化响应失败", "err", err) // 响应头已发出，只能记日志
	}
}

// Error 写出指定状态码和错误码的失败响应。
func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: errorObject{Code: code, Message: message}})
}

// ErrorFromDomain 把领域错误翻译成 HTTP。映射不到的算服务端错误：
// 对外只说"内部错误"，原文进日志，避免泄露 SQL 或堆栈。
func ErrorFromDomain(w http.ResponseWriter, r *http.Request, err error) {
	var status int
	var code string

	switch {
	case errors.Is(err, model.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, model.ErrSlugConflict):
		status, code = http.StatusConflict, "slug_conflict"
	case errors.Is(err, model.ErrParentNotFound):
		status, code = http.StatusBadRequest, "parent_not_found"
	case errors.Is(err, model.ErrCycle):
		status, code = http.StatusBadRequest, "cycle_detected"
	case errors.Is(err, model.ErrInvalidInput):
		status, code = http.StatusBadRequest, "bad_request"
	case errors.Is(err, model.ErrUnauthorized):
		status, code = http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, model.ErrWrongCredential):
		status, code = http.StatusForbidden, "wrong_credentials"
	default:
		slog.Error("未预期的错误", "path", r.URL.Path, "err", err)
		status, code = http.StatusInternalServerError, "internal_error"
		Error(w, status, code, "服务端内部错误")
		return
	}
	Error(w, status, code, err.Error())
}
