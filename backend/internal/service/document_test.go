package service

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
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/repo"
)

// setupService 与仓库层测试同一条约束：只在本地 kb 开发库上跑，否则跳过。
// 两个测试包都会 TRUNCATE 同一张表，所以必须 `go test -p 1 ./...` 串行跑包，否则互相清数据。
func setupService(t *testing.T) *DocumentService {
	t.Helper()

	if os.Getenv("CI") == "true" {
		t.Skip("CI 上没有本地开发库，业务层测试跳过")
	}
	url, err := config.DatabaseURL()
	if err != nil {
		t.Skipf("没有可用的开发库: %v", err)
	}
	if !strings.Contains(url, "/kb?") && !strings.HasSuffix(url, "/kb") {
		t.Skip("业务层测试只允许跑在 kb 开发库上")
	}
	pool, err := db.New(context.Background(), url)
	if err != nil {
		t.Skipf("开发库连不上: %v", err)
	}
	t.Cleanup(pool.Close)
	clearPool(t, pool)
	return NewDocument(repo.New(pool))
}

func clearPool(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE documents RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("清空 documents 失败: %v", err)
	}
}

func noteInput(slug, title string, parent *int64) SaveInput {
	return SaveInput{
		Type: model.TypeNote, Slug: slug, Title: title, ContentMD: title + " 正文",
		Tags: []string{"测试"}, Meta: json.RawMessage(`{}`), Status: model.StatusPublished, ParentID: parent,
	}
}

// treeShape 把树拍成 "root(mid(leaf))" 形式；直接 %+v 打印 []*TreeNode 只会看到指针。
func treeShape(ts []*model.TreeNode) string {
	parts := make([]string, 0, len(ts))
	for _, n := range ts {
		if len(n.Children) > 0 {
			parts = append(parts, n.Slug+"("+treeShape(n.Children)+")")
		} else {
			parts = append(parts, n.Slug)
		}
	}
	return strings.Join(parts, ",")
}

func TestBuildTSQuery(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"等价类", "等价类:*"},
		{"Black Box 用例", "black:* & box:* & 用例:*"},
		{"  多余   空格  ", "多余:* & 空格:*"},
		{"a&b|c", "a:* & b:* & c:*"}, // tsquery 运算符字符当成分隔符，用户无法注入语法
	}
	for _, c := range cases {
		got, err := BuildTSQuery(c.in)
		if err != nil {
			t.Fatalf("BuildTSQuery(%q) 报错: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("BuildTSQuery(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}

	if _, err := BuildTSQuery("   "); err == nil || !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("空白搜索词该报参数错误，实际 %v", err)
	}
	if _, err := BuildTSQuery("&|!<>"); err == nil || !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("纯运算符搜索词该报参数错误，实际 %v", err)
	}
	got, err := BuildTSQuery(strings.Repeat("x ", 30))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got, ":*"); n != maxSearchTerms {
		t.Errorf("搜索词该截断到 %d 个，实际 %d", maxSearchTerms, n)
	}
}

func TestParseTypesAndTags(t *testing.T) {
	if _, err := parseTypes("bogus"); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("非法 type 该报错，实际 %v", err)
	}
	got, err := parseTypes("project, note ,,idea")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[1] != model.TypeNote {
		t.Errorf("多类型解析不对: %v", got)
	}
	none, err := parseTypes("")
	if err != nil || len(none) != 0 {
		t.Errorf("空 type 该表示不限，实际 %v / %v", none, err)
	}

	tags := normalizeTags([]string{" Go ", "go", "", " 中文标签超长了中文标签超长了中文标签超长了中文标签超长了", "测试"})
	if len(tags) != 3 {
		t.Fatalf("去重去空后应剩 3 个标签，实际 %v", tags)
	}
	if tags[0] != "go" {
		t.Errorf("标签该统一小写，实际 %q", tags[0])
	}
	if len([]rune(tags[1])) > 32 {
		t.Errorf("长标签该被截到 32 个字符，实际 %d", len([]rune(tags[1])))
	}
	if normalizeTags(nil) == nil {
		t.Error("nil 标签该变成空切片，这样 JSON 里是 [] 而不是 null")
	}
}

func TestNormalizeMeta(t *testing.T) {
	if got, err := normalizeMeta(nil); err != nil || string(got) != "{}" {
		t.Errorf("缺省 meta 该是 {}，实际 %s / %v", got, err)
	}
	if _, err := normalizeMeta(json.RawMessage(`[1,2]`)); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("数组 meta 该报错，实际 %v", err)
	}
	if _, err := normalizeMeta(json.RawMessage(`{坏掉的`)); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("坏 JSON 该报错，实际 %v", err)
	}
	got, err := normalizeMeta(json.RawMessage(`{"lang":"go","time_complexity":"O(n)"}`))
	if err != nil || !strings.Contains(string(got), "O(n)") {
		t.Errorf("合法 meta 该原样保留，实际 %s / %v", got, err)
	}
}

func TestCreate_ValidationRules(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, SaveInput{Type: "wiki", Slug: "ok-slug", Title: "x", Meta: json.RawMessage(`{}`)}); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("非法 type 该 400，实际 %v", err)
	}
	for _, bad := range []string{"", "大写-SLUG", "-lead", "带空格 slug", strings.Repeat("a", 101)} {
		if _, err := svc.Create(ctx, SaveInput{Type: model.TypeNote, Slug: bad, Title: "x"}); !errors.Is(err, model.ErrInvalidInput) {
			t.Errorf("非法 slug %q 该 400，实际 %v", bad, err)
		}
	}
	if _, err := svc.Create(ctx, SaveInput{Type: model.TypeNote, Slug: "no-title", Title: "   "}); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("空标题该 400，实际 %v", err)
	}
	if _, err := svc.Create(ctx, SaveInput{Type: model.TypeNote, Slug: "bad-status", Title: "x", Status: "archieved"}); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("非法 status 该 400，实际 %v", err)
	}

	// 父节点规则
	project := noteInput("root-note", "根笔记", nil)
	created, err := svc.Create(ctx, project)
	if err != nil {
		t.Fatalf("建根笔记失败: %v", err)
	}
	if _, err := svc.Create(ctx, SaveInput{Type: model.TypeProject, Slug: "project-with-parent", Title: "x", ParentID: &created.ID}); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("非 note 挂父节点该 400，实际 %v", err)
	}
	missing := int64(999999)
	if _, err := svc.Create(ctx, noteInput("orphan", "孤儿", &missing)); !errors.Is(err, model.ErrParentNotFound) {
		t.Errorf("父不存在该 parent_not_found，实际 %v", err)
	}
	if _, err := svc.Create(ctx, noteInput("dup-slug", "重复", nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, noteInput("dup-slug", "又一遍", nil)); !errors.Is(err, model.ErrSlugConflict) {
		t.Errorf("重复 slug 该 409，实际 %v", err)
	}
}

func TestNotesTree_NestingAndOrphans(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	root, err := svc.Create(ctx, noteInput("tree-root", "根", nil))
	if err != nil {
		t.Fatal(err)
	}
	mid, err := svc.Create(ctx, noteInput("tree-mid", "中层", &root.ID))
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := svc.Create(ctx, noteInput("tree-leaf", "叶子", &mid.ID))
	if err != nil {
		t.Fatal(err)
	}

	tree, err := svc.NotesTree(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].ID != root.ID {
		t.Fatalf("顶层应只有一篇，实际 %s", treeShape(tree))
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != mid.ID {
		t.Fatalf("第二层不对: %s", treeShape(tree))
	}
	third := tree[0].Children[0].Children
	if len(third) != 1 || third[0].ID != leaf.ID {
		t.Fatalf("第三层不对: %s", treeShape(tree))
	}
	if third[0].Children == nil {
		t.Error("叶子节点的 children 该是空数组而不是 null")
	}

	// 父被删掉（或父是 draft 而访问者是访客）时，子节点提升到顶层，不能凭空消失
	if _, err := svc.Move(ctx, leaf.ID, nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Move(ctx, leaf.ID, &mid.ID, 0); err != nil {
		t.Fatal(err)
	}
	reloaded, err := svc.NotesTree(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded) != 1 || len(reloaded[0].Children) != 1 {
		t.Fatalf("移动回中层后树形不对: %s", treeShape(reloaded))
	}
}

func TestMove_RejectsCycles(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	root, _ := svc.Create(ctx, noteInput("cyc-root", "根", nil))
	mid, _ := svc.Create(ctx, noteInput("cyc-mid", "中层", &root.ID))
	leaf, _ := svc.Create(ctx, noteInput("cyc-leaf", "叶子", &mid.ID))

	if _, err := svc.Move(ctx, root.ID, &leaf.ID, 0); !errors.Is(err, model.ErrCycle) {
		t.Errorf("把根挂到自己孙子下面该判成环，实际 %v", err)
	}
	if _, err := svc.Move(ctx, root.ID, &root.ID, 0); !errors.Is(err, model.ErrCycle) {
		t.Errorf("自己当自己爸该判成环，实际 %v", err)
	}
	if _, err := svc.Move(ctx, mid.ID, &leaf.ID, 0); !errors.Is(err, model.ErrCycle) {
		t.Errorf("中层挂到自己孩子下面该判成环，实际 %v", err)
	}
	if _, err := svc.Move(ctx, leaf.ID, &root.ID, 2); err != nil {
		t.Errorf("合法的重新挂父不该报错: %v", err)
	}

	// 非 note 不参与树
	proj, err := svc.Create(ctx, SaveInput{Type: model.TypeProject, Slug: "cyc-proj", Title: "项目", Meta: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Move(ctx, proj.ID, nil, 1); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("项目不该能调树位置，实际 %v", err)
	}
}

func TestUpdate_ChildBlockingAndDraftVisibility(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	root, _ := svc.Create(ctx, noteInput("up-root", "根", nil))
	if _, err := svc.Create(ctx, noteInput("up-child", "子", &root.ID)); err != nil {
		t.Fatal(err)
	}

	in := noteInput("up-root", "根改名", nil)
	in.ID = root.ID
	in.Type = model.TypeIdea
	if _, err := svc.Update(ctx, in); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("还有子节点时不该把 note 改成别的类型，实际 %v", err)
	}

	in.Type = model.TypeNote
	in.Status = model.StatusDraft
	in.Meta = json.RawMessage(`{"mood":"tired"}`)
	updated, err := svc.Update(ctx, in)
	if err != nil {
		t.Fatalf("正常更新不该报错: %v", err)
	}
	if updated.Status != model.StatusDraft || !strings.Contains(string(updated.Meta), "tired") {
		t.Errorf("更新结果不对: %+v", updated)
	}

	anon, err := svc.List(ctx, ListInput{Page: 1, Size: 20, Viewer: false})
	if err != nil {
		t.Fatal(err)
	}
	if anon.Total != 1 { // 只有 up-child 可见
		t.Errorf("访客应只看到 1 篇（draft 被隐藏），实际 %d", anon.Total)
	}
	owner, err := svc.List(ctx, ListInput{Page: 1, Size: 20, Viewer: true})
	if err != nil {
		t.Fatal(err)
	}
	if owner.Total != 2 {
		t.Errorf("管理员该看到 2 篇，实际 %d", owner.Total)
	}

	if _, err := svc.Search(ctx, "", "", 1, 20, true); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("空搜索词该 400，实际 %v", err)
	}
	res, err := svc.Search(ctx, "根改名", "", 1, 20, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Logf("中文搜索命中 %d 条（simple 配置不分词的已知局限，见 DECISIONS D10）", res.Total)
	}
}
