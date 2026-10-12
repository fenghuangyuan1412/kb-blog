// Package model 定义跨层传递的数据结构和领域错误，不依赖 http 也不依赖 SQL。
package model

import (
	"encoding/json"
	"time"
)

// 四种内容类型，与数据库 CHECK 约束一致。
const (
	TypeProject   = "project"
	TypeAlgorithm = "algorithm"
	TypeIdea      = "idea"
	TypeNote      = "note"
)

func ValidType(t string) bool {
	switch t {
	case TypeProject, TypeAlgorithm, TypeIdea, TypeNote:
		return true
	}
	return false
}

// 两种发布状态。draft 只有带有效 Cookie 的请求能看到（见 DECISIONS D12）。
const (
	StatusPublished = "published"
	StatusDraft     = "draft"
)

func ValidStatus(s string) bool {
	return s == StatusPublished || s == StatusDraft
}

// Document 是一篇文档的完整形态，对应 docs/API.md 里的 Document。
type Document struct {
	ID          int64           `json:"id"`
	Type        string          `json:"type"`
	ParentID    *int64          `json:"parent_id"`
	Slug        string          `json:"slug"`
	Title       string          `json:"title"`
	Summary     string          `json:"summary"`
	ContentMD   string          `json:"content_md"`
	Tags        []string        `json:"tags"`
	Meta        json.RawMessage `json:"meta"`
	Status      string          `json:"status"`
	SortOrder   int             `json:"sort_order"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	ChildrenCnt int             `json:"children_count"`
}

// ListItem 是列表页用的精简形态，不含正文，对应 docs/API.md 里的 DocumentListItem。
type ListItem struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Tags      []string  `json:"tags"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListResult 是分页列表的响应体。
type ListResult struct {
	Items []ListItem `json:"items"`
	Total int        `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

// ListQuery 是 repo 查询列表所需的全部条件，已由 service 校验和归一化。
type ListQuery struct {
	Types        []string // 空表示不限类型
	Tag          string   // 空表示不按标签过滤
	TSQuery      string   // 空表示不做全文搜索；由 service 拼好
	IncludeDraft bool     // 已登录时为 true
	Page         int
	Size         int
}

// NoteRef 是笔记树节点的最小信息。Type 只在 service 校验父子关系时用，
// 接口输出用的是 TreeNode，不含这个字段。
type NoteRef struct {
	ID        int64  `json:"id"`
	Type      string `json:"-"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// TreeNode 是笔记树的输出节点，Children 为空数组而不是 null（前端可无脑 map）。
type TreeNode struct {
	ID        int64       `json:"id"`
	Slug      string      `json:"slug"`
	Title     string      `json:"title"`
	ParentID  *int64      `json:"parent_id"`
	SortOrder int         `json:"sort_order"`
	Children  []*TreeNode `json:"children"`
}

// TagCount 是一个标签及其文档数。
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}
