// Package handler 只做参数校验、调用 service、组装响应，不写业务规则也不碰 SQL。
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/render"
)

// 请求体上限：一篇长笔记的 Markdown 也远小于这个值，主要是防误操作和大包。
const maxBodyBytes = 1 << 20

// decodeJSON 严格解析请求体：未知字段直接报错（帮我早点发现写错的字段名），
// 并限制大小。失败时已经写好 400 响应，返回 false。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer func() { _, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 4<<10)) }() // 读完剩余 body 以便复用连接
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			render.Error(w, http.StatusBadRequest, "bad_request", "请求体过大（上限 1MB）")
			return false
		}
		if errors.Is(err, io.EOF) {
			render.Error(w, http.StatusBadRequest, "bad_request", "请求体不能为空")
			return false
		}
		render.Error(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("请求体 JSON 解析失败: %s", err))
		return false
	}
	return true
}

// parseID 把 URL 里的 {id} 转成 int64。
func parseID(w http.ResponseWriter, raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		render.Error(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("%s: id 必须是正整数，收到 %q", model.ErrInvalidInput, raw))
		return 0, false
	}
	return id, true
}

// parseIntParam 解析分页之类的查询参数，非法值返回 400。
func parseIntParam(w http.ResponseWriter, name, raw string) (int, bool) {
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		render.Error(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("%s: %s 必须是整数，收到 %q", model.ErrInvalidInput, name, raw))
		return 0, false
	}
	return n, true
}
