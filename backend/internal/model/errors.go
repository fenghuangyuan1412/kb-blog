package model

import "errors"

// 领域错误。handler 按这些哨兵值映射 HTTP 状态码和错误码（见 docs/API.md 第 0 节），
// 因此 service / repo 只许用 %w 包装它们，不许自己造字符串错误。
var (
	ErrNotFound        = errors.New("文档不存在")
	ErrSlugConflict    = errors.New("slug 已存在")
	ErrInvalidInput    = errors.New("参数不合法")
	ErrParentNotFound  = errors.New("父笔记不存在")
	ErrCycle           = errors.New("不能把笔记挂到自己或自己的子节点下")
	ErrUnauthorized    = errors.New("未登录")
	ErrWrongCredential = errors.New("邮箱或密码错误")
)
