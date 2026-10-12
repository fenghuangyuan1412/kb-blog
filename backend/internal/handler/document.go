package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/middleware"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/render"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/service"
)

type Document struct {
	svc *service.DocumentService
}

func NewDocument(svc *service.DocumentService) *Document {
	return &Document{svc: svc}
}

// documentRequest 对应 docs/API.md 第 3.1 / 3.2 的请求体。
type documentRequest struct {
	Type      string          `json:"type"`
	Slug      string          `json:"slug"`
	Title     string          `json:"title"`
	Summary   string          `json:"summary"`
	ContentMD string          `json:"content_md"`
	Tags      []string        `json:"tags"`
	Meta      json.RawMessage `json:"meta"`
	Status    string          `json:"status"`
	ParentID  *int64          `json:"parent_id"`
	SortOrder int             `json:"sort_order"`
}

func (r documentRequest) toInput(id int64) service.SaveInput {
	return service.SaveInput{
		ID:        id,
		Type:      r.Type,
		Slug:      r.Slug,
		Title:     r.Title,
		Summary:   r.Summary,
		ContentMD: r.ContentMD,
		Tags:      r.Tags,
		Meta:      r.Meta,
		Status:    r.Status,
		ParentID:  r.ParentID,
		SortOrder: r.SortOrder,
	}
}

// List 处理 GET /api/documents。draft 是否可见由登录状态决定，不给匿名请求开 include_draft 的口子。
func (h *Document) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, ok := parseIntParam(w, "page", query.Get("page"))
	if !ok {
		return
	}
	size, ok := parseIntParam(w, "size", query.Get("size"))
	if !ok {
		return
	}

	in := service.ListInput{
		Type:   query.Get("type"),
		Tag:    query.Get("tag"),
		Q:      query.Get("q"),
		Page:   page,
		Size:   size,
		Viewer: middleware.IsViewer(r.Context()),
	}
	result, err := h.svc.List(r.Context(), in)
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

// GetBySlug 处理 GET /api/documents/{slug}。
func (h *Document) GetBySlug(w http.ResponseWriter, r *http.Request) {
	doc, err := h.svc.GetBySlug(r.Context(), chi.URLParam(r, "slug"), middleware.IsViewer(r.Context()))
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, doc)
}

// NotesTree 处理 GET /api/notes/tree。
func (h *Document) NotesTree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.svc.NotesTree(r.Context(), middleware.IsViewer(r.Context()))
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, tree)
}

// Tags 处理 GET /api/tags。
func (h *Document) Tags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.svc.Tags(r.Context(), middleware.IsViewer(r.Context()))
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, tags)
}

// Search 处理 GET /api/search，复用列表查询。
func (h *Document) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, ok := parseIntParam(w, "page", query.Get("page"))
	if !ok {
		return
	}
	size, ok := parseIntParam(w, "size", query.Get("size"))
	if !ok {
		return
	}
	result, err := h.svc.Search(r.Context(), query.Get("q"), query.Get("type"), page, size, middleware.IsViewer(r.Context()))
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, result)
}

// Create 处理 POST /api/documents。
func (h *Document) Create(w http.ResponseWriter, r *http.Request) {
	var req documentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	doc, err := h.svc.Create(r.Context(), req.toInput(0))
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusCreated, doc)
}

// Update 处理 PUT /api/documents/{id}，全量覆盖。
func (h *Document) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req documentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	doc, err := h.svc.Update(r.Context(), req.toInput(id))
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, doc)
}

type deleteResponse struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

// Delete 处理 DELETE /api/documents/{id}，note 会级联删整棵子树。
func (h *Document) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	deleted, err := h.svc.Delete(r.Context(), id)
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, deleteResponse{ID: deleted, Deleted: true})
}

// Move 处理 POST /api/documents/{id}/move。
func (h *Document) Move(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req struct {
		ParentID  *int64 `json:"parent_id"`
		SortOrder int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	doc, err := h.svc.Move(r.Context(), id, req.ParentID, req.SortOrder)
	if err != nil {
		render.ErrorFromDomain(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, doc)
}
