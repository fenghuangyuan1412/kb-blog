package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/repo"
)

type DocumentService struct {
	repo *repo.Repo
}

func NewDocument(r *repo.Repo) *DocumentService {
	return &DocumentService{repo: r}
}

// ListInput 是 handler 解析查询参数后交给 service 的原始输入。
type ListInput struct {
	Type   string // 逗号分隔的多类型，空表示不限
	Tag    string
	Q      string
	Page   int
	Size   int
	Viewer bool // 已登录时能看到 draft
}

type SaveInput struct {
	ID        int64
	Type      string
	Slug      string
	Title     string
	Summary   string
	ContentMD string
	Tags      []string
	Meta      json.RawMessage
	Status    string
	ParentID  *int64
	SortOrder int
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxSearchTerms  = 8
	slugPattern     = `^[a-z0-9][a-z0-9_-]{0,99}$`
)

var slugRE = regexp.MustCompile(slugPattern)

// List 校验分页与过滤条件，把搜索词转成 tsquery，再交给 repo。
func (s *DocumentService) List(ctx context.Context, in ListInput) (model.ListResult, error) {
	types, err := parseTypes(in.Type)
	if err != nil {
		return model.ListResult{}, err
	}

	page := in.Page
	if page < 1 {
		page = 1
	}
	size := in.Size
	if size <= 0 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	q := model.ListQuery{
		Types:        types,
		Tag:          normalizeTag(in.Tag),
		IncludeDraft: in.Viewer,
		Page:         page,
		Size:         size,
	}
	if strings.TrimSpace(in.Q) != "" {
		tsq, err := BuildTSQuery(in.Q)
		if err != nil {
			return model.ListResult{}, err
		}
		q.TSQuery = tsq
	}
	return s.repo.List(ctx, q)
}

func (s *DocumentService) GetBySlug(ctx context.Context, slug string, viewer bool) (model.Document, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return model.Document{}, fmt.Errorf("%w: slug 不能为空", model.ErrInvalidInput)
	}
	return s.repo.GetBySlug(ctx, slug, viewer)
}

// NotesTree 一次取回全部笔记，在内存里组装成嵌套结构（DECISIONS D11）。
// 父节点查不到（比如父是 draft 而访问者是访客）时，把该节点当作顶层，避免整棵子树凭空消失。
func (s *DocumentService) NotesTree(ctx context.Context, viewer bool) ([]*model.TreeNode, error) {
	refs, err := s.repo.ListNoteRefs(ctx, viewer)
	if err != nil {
		return nil, err
	}

	nodes := make(map[int64]*model.TreeNode, len(refs))
	for _, r := range refs {
		nodes[r.ID] = &model.TreeNode{ID: r.ID, Slug: r.Slug, Title: r.Title, ParentID: r.ParentID, SortOrder: r.SortOrder, Children: []*model.TreeNode{}}
	}

	roots := []*model.TreeNode{}
	for _, r := range refs {
		node := nodes[r.ID]
		parent, hasParent := r.ParentID, false
		if parent != nil {
			if p, ok := nodes[*parent]; ok && *parent != r.ID {
				hasParent = true
				p.Children = append(p.Children, node)
			}
		}
		if !hasParent {
			node.ParentID = nil
			roots = append(roots, node)
		}
	}
	return roots, nil
}

func (s *DocumentService) Tags(ctx context.Context, viewer bool) ([]model.TagCount, error) {
	return s.repo.ListTags(ctx, viewer)
}

// Search 与 List 共用一套查询，只是必须有搜索词（见 docs/API.md 1.5）。
func (s *DocumentService) Search(ctx context.Context, q, typeFilter string, page, size int, viewer bool) (model.ListResult, error) {
	if strings.TrimSpace(q) == "" {
		return model.ListResult{}, fmt.Errorf("%w: 搜索必须带 q", model.ErrInvalidInput)
	}
	return s.List(ctx, ListInput{Type: typeFilter, Q: q, Page: page, Size: size, Viewer: viewer})
}

// Create 校验后写入。slug 冲突由 repo 转成 ErrSlugConflict。
func (s *DocumentService) Create(ctx context.Context, in SaveInput) (model.Document, error) {
	doc, err := s.normalize(ctx, in, false)
	if err != nil {
		return model.Document{}, err
	}
	return s.repo.Insert(ctx, doc)
}

// Update 是全量覆盖：请求体没带的正文字段按空值处理（见 docs/API.md 3.2）。
func (s *DocumentService) Update(ctx context.Context, in SaveInput) (model.Document, error) {
	if in.ID <= 0 {
		return model.Document{}, fmt.Errorf("%w: id 必须是正整数", model.ErrInvalidInput)
	}
	doc, err := s.normalize(ctx, in, true)
	if err != nil {
		return model.Document{}, err
	}
	return s.repo.Update(ctx, doc)
}

// Delete 返回被删掉的文档 id；note 会连带删除整棵子树。
func (s *DocumentService) Delete(ctx context.Context, id int64) (int64, error) {
	if id <= 0 {
		return 0, fmt.Errorf("%w: id 必须是正整数", model.ErrInvalidInput)
	}
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return 0, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return 0, err
	}
	return id, nil
}

// Move 改父节点和同级排序。除了父子合法性，还要防成环。
func (s *DocumentService) Move(ctx context.Context, id int64, parentID *int64, sortOrder int) (model.Document, error) {
	if id <= 0 {
		return model.Document{}, fmt.Errorf("%w: id 必须是正整数", model.ErrInvalidInput)
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.Document{}, err
	}
	if current.Type != model.TypeNote {
		return model.Document{}, fmt.Errorf("%w: 只有 note 可以设置父子关系", model.ErrInvalidInput)
	}
	if parentID != nil {
		if err := s.checkParent(ctx, *parentID); err != nil {
			return model.Document{}, err
		}
		if err := s.checkCycle(ctx, id, *parentID); err != nil {
			return model.Document{}, err
		}
	}
	return s.repo.Move(ctx, id, parentID, sortOrder)
}

// normalize 把所有字段校验和归一化收在一处，Create / Update 走同一条路。
func (s *DocumentService) normalize(ctx context.Context, in SaveInput, isUpdate bool) (model.Document, error) {
	if !model.ValidType(in.Type) {
		return model.Document{}, fmt.Errorf("%w: type 只能是 project/algorithm/idea/note", model.ErrInvalidInput)
	}
	if !slugRE.MatchString(in.Slug) {
		return model.Document{}, fmt.Errorf("%w: slug 必须是小写字母数字开头、只含 a-z0-9_- 、长度 1-100", model.ErrInvalidInput)
	}
	if strings.TrimSpace(in.Title) == "" {
		return model.Document{}, fmt.Errorf("%w: title 不能为空", model.ErrInvalidInput)
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = model.StatusPublished
	}
	if !model.ValidStatus(status) {
		return model.Document{}, fmt.Errorf("%w: status 只能是 published/draft", model.ErrInvalidInput)
	}
	meta, err := normalizeMeta(in.Meta)
	if err != nil {
		return model.Document{}, err
	}
	tags := normalizeTags(in.Tags)

	if in.ParentID != nil && in.Type != model.TypeNote {
		return model.Document{}, fmt.Errorf("%w: 只有 note 能挂在笔记树里", model.ErrInvalidInput)
	}
	if in.ParentID != nil {
		if err := s.checkParent(ctx, *in.ParentID); err != nil {
			return model.Document{}, err
		}
		if isUpdate && *in.ParentID == in.ID {
			return model.Document{}, model.ErrCycle
		}
	}
	// 把有子节点的 note 改成别的类型会破坏树的完整性
	if isUpdate && in.Type != model.TypeNote {
		existing, err := s.repo.GetByID(ctx, in.ID)
		if err != nil {
			return model.Document{}, err
		}
		if existing.Type == model.TypeNote {
			n, err := s.repo.CountChildren(ctx, in.ID)
			if err != nil {
				return model.Document{}, err
			}
			if n > 0 {
				return model.Document{}, fmt.Errorf("%w: 该笔记还有 %d 个子节点，先移走再改类型", model.ErrInvalidInput, n)
			}
		}
	}

	return model.Document{
		ID:        in.ID,
		Type:      in.Type,
		ParentID:  in.ParentID,
		Slug:      in.Slug,
		Title:     strings.TrimSpace(in.Title),
		Summary:   in.Summary,
		ContentMD: in.ContentMD,
		Tags:      tags,
		Meta:      meta,
		Status:    status,
		SortOrder: in.SortOrder,
	}, nil
}

func (s *DocumentService) checkParent(ctx context.Context, parentID int64) error {
	parent, err := s.repo.GetNoteRefByID(ctx, parentID)
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: id=%d", model.ErrParentNotFound, parentID)
		}
		return err
	}
	if parent.Type != model.TypeNote {
		return fmt.Errorf("%w: 父节点必须是 note", model.ErrParentNotFound)
	}
	return nil
}

// checkCycle 从新父节点往上走到根，途中遇到自己就是成环。
// 笔记量级下多走几次单行查询比引入递归 CTE 划算（DECISIONS D11）。
func (s *DocumentService) checkCycle(ctx context.Context, id, parentID int64) error {
	current := parentID
	for depth := 0; depth < 64; depth++ {
		if current == id {
			return model.ErrCycle
		}
		ref, err := s.repo.GetNoteRefByID(ctx, current)
		if err != nil {
			if isNotFound(err) {
				return fmt.Errorf("%w: id=%d", model.ErrParentNotFound, current)
			}
			return err
		}
		if ref.ParentID == nil {
			return nil
		}
		current = *ref.ParentID
	}
	return fmt.Errorf("%w: 笔记层级超过 64 层，数据可能已损坏", model.ErrInvalidInput)
}

func isNotFound(err error) bool {
	return errors.Is(err, model.ErrNotFound)
}

// BuildTSQuery 把用户搜索词转成 tsquery 语法字符串。
// 规则：按空白切词，去掉 tsquery 的运算符字符，每个词加 :* 前缀匹配，词之间 AND。
// 中文局限见 DECISIONS D10。
func BuildTSQuery(raw string) (string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("&|!<>:()*'\"\\", r)
	})

	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(strings.ToLower(f))
		if f == "" {
			continue
		}
		terms = append(terms, f)
		if len(terms) == maxSearchTerms {
			break
		}
	}
	if len(terms) == 0 {
		return "", fmt.Errorf("%w: 搜索词没有有效内容", model.ErrInvalidInput)
	}
	for i, t := range terms {
		terms[i] = t + ":*"
	}
	return strings.Join(terms, " & "), nil
}

func parseTypes(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !model.ValidType(p) {
			return nil, fmt.Errorf("%w: 未知的 type: %s", model.ErrInvalidInput, p)
		}
		out = append(out, p)
	}
	return out, nil
}

func normalizeTag(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "*" {
		return ""
	}
	return s
}

// normalizeTags 去空、去重、转小写、限长度。标签统一小写存库（DATA_MODEL 第 2 节）。
func normalizeTags(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, t := range in {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" || seen[t] {
			continue
		}
		if len([]rune(t)) > 32 {
			t = string([]rune(t)[:32])
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == 20 {
			break
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// normalizeMeta 只要求 meta 是合法 JSON 对象，类型专属字段由前端按约定读（DATA_MODEL 第 3 节）。
func normalizeMeta(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return json.RawMessage(`{}`), nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return nil, fmt.Errorf("%w: meta 必须是 JSON 对象: %s", model.ErrInvalidInput, trimmed)
	}
	encoded, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("序列化 meta 失败: %w", err)
	}
	return encoded, nil
}
