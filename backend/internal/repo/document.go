// Package repo 只做 SQL 和行到结构体的映射，不含业务判断。
package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
)

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// documents 的全部业务列（不含生成列 search_vec）。
const docCols = `id, type, parent_id, slug, title, summary, content_md, tags, meta,
	status, sort_order, created_at, updated_at`

// 列表页不返回正文和 meta，列顺序必须和 List 里的 Scan 顺序一致。
const listCols = `id, type, slug, title, summary, tags, status, created_at, updated_at`

// 详情查询把子节点数量一起带出来，省一次往返。
const detailSelect = `SELECT ` + docCols + `,
	(SELECT COUNT(*) FROM documents c WHERE c.parent_id = documents.id)::int AS children_count
	FROM documents`

// List 按条件分页查询。TSQuery 已由 service 拼好，这里只负责执行。
func (r *Repo) List(ctx context.Context, q model.ListQuery) (model.ListResult, error) {
	types := q.Types
	if len(types) == 0 {
		types = []string{}
	}
	var tag, tsq any
	if q.Tag != "" {
		tag = q.Tag
	}
	if q.TSQuery != "" {
		tsq = q.TSQuery
	}

	sqlText := `
		SELECT ` + listCols + `, COUNT(*) OVER() AS total_count
		FROM documents
		WHERE ($1::text[] = '{}'::text[] OR type = ANY($1::text[]))
		  AND ($2::text IS NULL OR $2 = ANY(tags))
		  AND ($3::text IS NULL OR search_vec @@ to_tsquery('simple', $3))
		  AND ($4 OR status = 'published')
		ORDER BY sort_order ASC, created_at DESC
		LIMIT $5 OFFSET $6`

	rows, err := r.pool.Query(ctx, sqlText, types, tag, tsq, q.IncludeDraft, q.Size, (q.Page-1)*q.Size)
	if err != nil {
		return model.ListResult{}, fmt.Errorf("查询文档列表失败: %w", err)
	}
	defer rows.Close()

	result := model.ListResult{Items: []model.ListItem{}, Page: q.Page, Size: q.Size}
	for rows.Next() {
		var li model.ListItem
		if err := rows.Scan(&li.ID, &li.Type, &li.Slug, &li.Title, &li.Summary,
			&li.Tags, &li.Status, &li.CreatedAt, &li.UpdatedAt, &result.Total); err != nil {
			return model.ListResult{}, fmt.Errorf("读取文档列表行失败: %w", err)
		}
		if li.Tags == nil {
			li.Tags = []string{}
		}
		result.Items = append(result.Items, li)
	}
	if err := rows.Err(); err != nil {
		return model.ListResult{}, fmt.Errorf("遍历文档列表失败: %w", err)
	}
	return result, nil
}

func (r *Repo) GetBySlug(ctx context.Context, slug string, includeDraft bool) (model.Document, error) {
	row := r.pool.QueryRow(ctx, detailSelect+` WHERE slug = $1 AND ($2 OR status = 'published')`, slug, includeDraft)
	return scanDetail(row, "按 slug 查询文档")
}

func (r *Repo) GetByID(ctx context.Context, id int64) (model.Document, error) {
	row := r.pool.QueryRow(ctx, detailSelect+` WHERE id = $1`, id)
	return scanDetail(row, "按 id 查询文档")
}

// ListNoteRefs 一次取回全部笔记，供 service 在内存里组装树（DECISIONS D11）。
func (r *Repo) ListNoteRefs(ctx context.Context, includeDraft bool) ([]model.NoteRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, type, slug, title, parent_id, sort_order
		FROM documents
		WHERE type = 'note' AND ($1 OR status = 'published')
		ORDER BY sort_order ASC, title ASC`, includeDraft)
	if err != nil {
		return nil, fmt.Errorf("查询笔记列表失败: %w", err)
	}
	defer rows.Close()

	refs := []model.NoteRef{}
	for rows.Next() {
		var n model.NoteRef
		if err := rows.Scan(&n.ID, &n.Type, &n.Slug, &n.Title, &n.ParentID, &n.SortOrder); err != nil {
			return nil, fmt.Errorf("读取笔记行失败: %w", err)
		}
		refs = append(refs, n)
	}
	return refs, rows.Err()
}

func (r *Repo) GetNoteRefByID(ctx context.Context, id int64) (model.NoteRef, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, type, slug, title, parent_id, sort_order
		FROM documents WHERE id = $1`, id)
	var n model.NoteRef
	if err := row.Scan(&n.ID, &n.Type, &n.Slug, &n.Title, &n.ParentID, &n.SortOrder); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.NoteRef{}, fmt.Errorf("%w: id=%d", model.ErrNotFound, id)
		}
		return model.NoteRef{}, fmt.Errorf("读取笔记 %d 失败: %w", id, err)
	}
	return n, nil
}

func (r *Repo) CountChildren(ctx context.Context, id int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM documents WHERE parent_id = $1`, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计 %d 的子节点失败: %w", id, err)
	}
	return n, nil
}

// ListTags 展开 tags 数组统计每个标签的文档数。
func (r *Repo) ListTags(ctx context.Context, includeDraft bool) ([]model.TagCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.tag, COUNT(*)::int AS count
		FROM documents d, unnest(d.tags) AS t(tag)
		WHERE ($1 OR d.status = 'published')
		GROUP BY t.tag
		ORDER BY count DESC, t.tag ASC`, includeDraft)
	if err != nil {
		return nil, fmt.Errorf("统计标签失败: %w", err)
	}
	defer rows.Close()

	tags := []model.TagCount{}
	for rows.Next() {
		var t model.TagCount
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, fmt.Errorf("读取标签行失败: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// insertSQL 用 CTE 把写入和详情读合成一条语句，直接拿到 children_count。
const insertSQL = `
	WITH written AS (
		INSERT INTO documents (type, parent_id, slug, title, summary, content_md, tags, meta, status, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8::jsonb, '{}'::jsonb), $9, $10)
		RETURNING *
	)
	SELECT ` + docCols + `,
		(SELECT COUNT(*) FROM documents c WHERE c.parent_id = written.id)::int AS children_count
	FROM written`

const updateSQL = `
	WITH written AS (
		UPDATE documents
		SET type = $2, parent_id = $3, slug = $4, title = $5, summary = $6, content_md = $7,
		    tags = $8, meta = COALESCE($9::jsonb, '{}'::jsonb), status = $10, sort_order = $11, updated_at = now()
		WHERE id = $1
		RETURNING *
	)
	SELECT ` + docCols + `,
		(SELECT COUNT(*) FROM documents c WHERE c.parent_id = written.id)::int AS children_count
	FROM written`

const moveSQL = `
	WITH written AS (
		UPDATE documents SET parent_id = $2, sort_order = $3, updated_at = now()
		WHERE id = $1
		RETURNING *
	)
	SELECT ` + docCols + `,
		(SELECT COUNT(*) FROM documents c WHERE c.parent_id = written.id)::int AS children_count
	FROM written`

func (r *Repo) Insert(ctx context.Context, d model.Document) (model.Document, error) {
	d = forWrite(d)
	row := r.pool.QueryRow(ctx, insertSQL, d.Type, d.ParentID, d.Slug, d.Title, d.Summary,
		d.ContentMD, d.Tags, d.Meta, d.Status, d.SortOrder)
	return scanDetail(row, "新建文档")
}

func (r *Repo) Update(ctx context.Context, d model.Document) (model.Document, error) {
	d = forWrite(d)
	row := r.pool.QueryRow(ctx, updateSQL, d.ID, d.Type, d.ParentID, d.Slug, d.Title, d.Summary,
		d.ContentMD, d.Tags, d.Meta, d.Status, d.SortOrder)
	return scanDetail(row, "更新文档")
}

func (r *Repo) Move(ctx context.Context, id int64, parentID *int64, sortOrder int) (model.Document, error) {
	row := r.pool.QueryRow(ctx, moveSQL, id, parentID, sortOrder)
	return scanDetail(row, "移动文档")
}

// Delete 删除文档；note 会级联删除整棵子树（数据库 ON DELETE CASCADE）。
func (r *Repo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除文档 %d 失败: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: id=%d", model.ErrNotFound, id)
	}
	return nil
}

// forWrite 兜住 NOT NULL 列：tags / meta 不允许写 NULL。
// service 正常路径已经归一化过，这里防的是 seed、测试等直接调 repo 的入口。
func forWrite(d model.Document) model.Document {
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if len(d.Meta) == 0 {
		d.Meta = json.RawMessage(`{}`)
	}
	return d
}

func scanDetail(row pgx.Row, op string) (model.Document, error) {
	var d model.Document
	err := row.Scan(&d.ID, &d.Type, &d.ParentID, &d.Slug, &d.Title, &d.Summary, &d.ContentMD,
		&d.Tags, &d.Meta, &d.Status, &d.SortOrder, &d.CreatedAt, &d.UpdatedAt, &d.ChildrenCnt)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// 领域错误不加工，保证对外 message 干净
			return model.Document{}, model.ErrNotFound
		case isSlugConflict(err):
			return model.Document{}, model.ErrSlugConflict
		default:
			return model.Document{}, fmt.Errorf("%s失败: %w", op, err)
		}
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if len(d.Meta) == 0 {
		d.Meta = []byte(`{}`)
	}
	return d, nil
}

// isSlugConflict 识别唯一约束冲突。documents 上只有 slug 是唯一约束，
// 所以 23505 只可能是 slug 撞了。
func isSlugConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
