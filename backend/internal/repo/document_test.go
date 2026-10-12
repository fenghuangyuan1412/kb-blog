package repo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/config"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/db"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
)

// 这些是针对真实 PostgreSQL 的仓库层测试（全文搜索、数组列、生成列都躲不过真库）。
// 只允许连名字为 kb 的开发库，其它情况直接跳过，绝不在别的环境上跑 TRUNCATE。
// 与业务层测试共用这张表，跑测试要用 `go test -p 1 ./...` 把包串行化。
func setup(t *testing.T) *Repo {
	t.Helper()

	if os.Getenv("CI") == "true" {
		t.Skip("CI 上没有本地开发库，仓库层测试跳过（冒烟测试覆盖同样的链路）")
	}
	url, err := config.DatabaseURL()
	if err != nil {
		t.Skipf("没有可用的开发库: %v", err)
	}
	if !strings.Contains(url, "/kb?") && !strings.HasSuffix(url, "/kb") {
		t.Skipf("仓库层测试只允许跑在 kb 开发库上，当前指向另一个库")
	}

	ctx := context.Background()
	pool, err := db.New(ctx, url)
	if err != nil {
		t.Skipf("开发库连不上: %v", err)
	}
	t.Cleanup(pool.Close)

	truncate(t, pool)
	return New(pool)
}

func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE documents RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空 documents 失败: %v", err)
	}
}

func newNote(t *testing.T, r *Repo, slug, title string, parentID *int64, tags ...string) model.Document {
	t.Helper()
	doc, err := r.Insert(context.Background(), model.Document{
		Type: model.TypeNote, Slug: slug, Title: title, Summary: "", ContentMD: title + " 的正文",
		Tags: tags, Meta: json.RawMessage(`{}`), Status: model.StatusPublished, ParentID: parentID,
	})
	if err != nil {
		t.Fatalf("插入笔记 %s 失败: %v", slug, err)
	}
	return doc
}

func TestList_EmptyResultIsNotEmptyError(t *testing.T) {
	r := setup(t)

	got, err := r.List(context.Background(), model.ListQuery{Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("空表查询不该报错: %v", err)
	}
	if got.Total != 0 || len(got.Items) != 0 {
		t.Fatalf("期望 0 条，实际 total=%d items=%d", got.Total, len(got.Items))
	}
	if got.Items == nil {
		t.Error("items 应该是空切片而不是 null，前端要无脑 map")
	}
}

func TestList_TypeFilterAndPagination(t *testing.T) {
	r := setup(t)
	newNote(t, r, "n1", "第一页", nil, "测试")
	newNote(t, r, "n2", "第二页", nil, "测试")
	proj, err := r.Insert(context.Background(), model.Document{
		Type: model.TypeProject, Slug: "p1", Title: "项目一", Tags: []string{"go"},
		Meta: json.RawMessage(`{"repo":"x"}`), Status: model.StatusPublished,
	})
	if err != nil {
		t.Fatalf("插入项目失败: %v", err)
	}

	all, err := r.List(context.Background(), model.ListQuery{Page: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 3 {
		t.Errorf("不加过滤应有 3 条，实际 %d", all.Total)
	}

	notes, err := r.List(context.Background(), model.ListQuery{Types: []string{model.TypeNote}, Page: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if notes.Total != 2 {
		t.Errorf("note 应有 2 条，实际 %d", notes.Total)
	}
	for _, it := range notes.Items {
		if it.Type != model.TypeNote {
			t.Errorf("type 过滤漏了: %v", it)
		}
		encoded, err := json.Marshal(it)
		if err != nil {
			t.Fatalf("序列化列表项失败: %v", err)
		}
		if strings.Contains(string(encoded), "content_md") {
			t.Errorf("列表项不该带正文，实际 %s", encoded)
		}
	}

	page2, err := r.List(context.Background(), model.ListQuery{Page: 2, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page2.Total != 3 || len(page2.Items) != 1 {
		t.Errorf("第 2 页应剩 1 条（total 仍是 3），实际 items=%d total=%d", len(page2.Items), page2.Total)
	}

	byTag, err := r.List(context.Background(), model.ListQuery{Tag: "测试", Page: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if byTag.Total != 2 {
		t.Errorf("tag=测试 应有 2 条，实际 %d", byTag.Total)
	}

	_ = proj
}

func TestList_FullTextSearchUsesIndexNotLike(t *testing.T) {
	r := setup(t)
	newNote(t, r, "equiv", "等价类划分", nil, "测试")
	newNote(t, r, "bva", "边界值分析", nil, "测试")

	res, err := r.List(context.Background(), model.ListQuery{TSQuery: "等价:*", Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("全文搜索失败: %v", err)
	}
	if res.Total != 1 || res.Items[0].Slug != "equiv" {
		t.Fatalf("期望只命中等价类那条，实际 %+v", res.Items)
	}

	miss, err := r.List(context.Background(), model.ListQuery{TSQuery: "不存在的词:*", Page: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if miss.Total != 0 {
		t.Errorf("不存在的词应该 0 命中，实际 %d", miss.Total)
	}
}

func TestList_DraftVisibility(t *testing.T) {
	r := setup(t)
	if _, err := r.Insert(context.Background(), model.Document{
		Type: model.TypeIdea, Slug: "half-baked", Title: "没想明白", Status: model.StatusDraft,
		Tags: []string{}, Meta: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	anon, err := r.List(context.Background(), model.ListQuery{Page: 1, Size: 20, IncludeDraft: false})
	if err != nil {
		t.Fatal(err)
	}
	if anon.Total != 0 {
		t.Errorf("访客不该看到 draft，实际 %d 条", anon.Total)
	}
	owner, err := r.List(context.Background(), model.ListQuery{Page: 1, Size: 20, IncludeDraft: true})
	if err != nil {
		t.Fatal(err)
	}
	if owner.Total != 1 {
		t.Errorf("管理员该看到 draft，实际 %d 条", owner.Total)
	}
}

func TestGetBySlug_AndChildrenCount(t *testing.T) {
	r := setup(t)
	root := newNote(t, r, "root", "父", nil)
	newNote(t, r, "kid1", "子一", &root.ID)
	newNote(t, r, "kid2", "子二", &root.ID)

	got, err := r.GetBySlug(context.Background(), "root", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChildrenCnt != 2 {
		t.Errorf("children_count 应为 2，实际 %d", got.ChildrenCnt)
	}
	if string(got.Meta) != "{}" {
		t.Errorf("meta 默认值应为 {}，实际 %s", got.Meta)
	}

	if _, err := r.GetBySlug(context.Background(), "ghost", false); err == nil {
		t.Error("查不存在的 slug 该报错")
	} else if !isModelErr(err, model.ErrNotFound) {
		t.Errorf("该返回 ErrNotFound，实际 %v", err)
	}
}

func TestInsert_SlugConflictBecomesDomainError(t *testing.T) {
	r := setup(t)
	newNote(t, r, "dup", "第一条", nil)

	_, err := r.Insert(context.Background(), model.Document{
		Type: model.TypeNote, Slug: "dup", Title: "第二条", Tags: []string{}, Meta: json.RawMessage(`{}`),
		Status: model.StatusPublished,
	})
	if err == nil {
		t.Fatal("重复 slug 该报错")
	}
	if !isModelErr(err, model.ErrSlugConflict) {
		t.Errorf("该转成 ErrSlugConflict，实际 %v", err)
	}
	if strings.Contains(err.Error(), "documents_slug_key") || strings.Contains(err.Error(), "INSERT") {
		t.Errorf("不该把数据库细节漏进领域错误: %v", err)
	}
}

func TestUpdateAndMove(t *testing.T) {
	r := setup(t)
	a := newNote(t, r, "a", "A", nil)
	b := newNote(t, r, "b", "B", nil)

	updated, err := r.Update(context.Background(), model.Document{
		ID: a.ID, Type: model.TypeNote, Slug: "a-renamed", Title: "A 改名", Summary: "摘要",
		ContentMD: "正文", Tags: []string{"测试"}, Meta: json.RawMessage(`{"mood":"ok"}`),
		Status: model.StatusDraft, SortOrder: 5,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Slug != "a-renamed" || updated.Status != model.StatusDraft || updated.SortOrder != 5 {
		t.Errorf("更新结果不对: %+v", updated)
	}
	if updated.UpdatedAt.Before(updated.CreatedAt) {
		t.Error("updated_at 没被刷新")
	}

	moved, err := r.Move(context.Background(), a.ID, &b.ID, 3)
	if err != nil {
		t.Fatalf("移动失败: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != b.ID || moved.SortOrder != 3 {
		t.Errorf("移动后父子不对: %+v", moved)
	}

	if _, err := r.Move(context.Background(), 999999, nil, 0); !isModelErr(err, model.ErrNotFound) {
		t.Errorf("移动不存在的文档该 404，实际 %v", err)
	}
}

func TestDelete_AndCountChildren(t *testing.T) {
	r := setup(t)
	root := newNote(t, r, "del-root", "要被删", nil)
	child := newNote(t, r, "del-child", "跟着一起没了", &root.ID)

	n, err := r.CountChildren(context.Background(), root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("子节点数应为 1，实际 %d", n)
	}

	if err := r.Delete(context.Background(), root.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := r.GetByID(context.Background(), child.ID); !isModelErr(err, model.ErrNotFound) {
		t.Errorf("删父节点该级联删子节点，实际 %v", err)
	}
	if err := r.Delete(context.Background(), root.ID); !isModelErr(err, model.ErrNotFound) {
		t.Errorf("重复删除该 404，实际 %v", err)
	}
}

func TestListNoteRefsAndTags(t *testing.T) {
	r := setup(t)
	root := newNote(t, r, "t-root", "根", nil, "Go", " 测试 ")
	newNote(t, r, "t-child", "子", &root.ID, "go")

	refs, err := r.ListNoteRefs(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("应有 2 条笔记，实际 %d", len(refs))
	}
	if refs[0].Type != model.TypeNote {
		t.Errorf("NoteRef 该带 type 供 service 校验，实际 %q", refs[0].Type)
	}

	tags, err := r.ListTags(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) == 0 {
		t.Fatal("标签统计不该为空")
	}
	t.Logf("标签统计: %+v", tags)
}

func isModelErr(err, target error) bool {
	return errors.Is(err, target)
}
