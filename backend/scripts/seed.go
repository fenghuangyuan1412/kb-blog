// Command seed 往开发库灌一批演示内容：四种类型各一篇，外加一棵三层笔记树（含一篇草稿）。
// 幂等做法是先 TRUNCATE 再插入，所以重复跑结果一致。
//
//	go run ./scripts           只允许 kb 开发库
//	go run ./scripts -force    明确知道会清空 documents 表，才往别的库灌数据
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/config"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/db"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/repo"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/service"
)

// seedDoc 用 slug 引用父节点，声明顺序保证父先于子创建。
type seedDoc struct {
	service.SaveInput
	Parent string
}

func main() {
	force := flag.Bool("force", false, "允许对非 kb 开发库执行（会清空 documents 表）")
	flag.Parse()

	ctx := context.Background()
	url, err := config.DatabaseURL()
	if err != nil {
		exitf("读取 DATABASE_URL 失败: %v", err)
	}
	if !isDevDB(url) && !*force {
		exitf("DATABASE_URL 指向的不是 kb 开发库（%s），拒绝执行；确认要清空目标库请加 -force", maskDSN(url))
	}

	pool, err := db.New(ctx, url)
	if err != nil {
		exitf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `TRUNCATE documents RESTART IDENTITY CASCADE`); err != nil {
		exitf("清空 documents 失败: %v", err)
	}

	svc := service.NewDocument(repo.New(pool))
	ids := make(map[string]int64)
	for _, it := range items() {
		doc, err := svc.Create(ctx, withParent(it, ids))
		if err != nil {
			exitf("创建 %s 失败: %v", it.Slug, err)
		}
		ids[it.Slug] = doc.ID
		fmt.Printf("  + [%-9s] %s (id=%d)\n", doc.Type, doc.Slug, doc.ID)
	}

	res, err := svc.List(ctx, service.ListInput{Size: 100, Viewer: true})
	if err != nil {
		exitf("统计失败: %v", err)
	}
	fmt.Printf("\n共 %d 篇，其中草稿 %d 篇\n", res.Total, countDraft(res.Items))

	tree, err := svc.NotesTree(ctx, true)
	if err != nil {
		exitf("读笔记树失败: %v", err)
	}
	fmt.Println("笔记树：")
	printTree(tree, "  ")
}

func withParent(it seedDoc, ids map[string]int64) service.SaveInput {
	in := it.SaveInput
	if it.Parent == "" {
		return in
	}
	id, ok := ids[it.Parent]
	if !ok {
		exitf("%s 的父节点 %s 还没创建，检查声明顺序", it.Slug, it.Parent)
	}
	in.ParentID = &id
	return in
}

func isDevDB(url string) bool {
	return strings.Contains(url, "/kb?") || strings.HasSuffix(url, "/kb")
}

// 连接串要打到终端，密码先遮掉。
func maskDSN(url string) string {
	i, j := strings.Index(url, "//"), strings.LastIndex(url, "@")
	if j > i {
		return url[:i+2] + "***" + url[j:]
	}
	return url
}

func countDraft(items []model.ListItem) int {
	n := 0
	for _, it := range items {
		if it.Status == model.StatusDraft {
			n++
		}
	}
	return n
}

func printTree(nodes []*model.TreeNode, indent string) {
	for _, n := range nodes {
		fmt.Printf("%s%s / %s\n", indent, n.Slug, n.Title)
		printTree(n.Children, indent+"  ")
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func items() []seedDoc {
	return []seedDoc{
		{SaveInput: service.SaveInput{
			Type:      model.TypeProject,
			Slug:      "kb-blog",
			Title:     "个人博客 / 知识库系统",
			Summary:   "把项目笔记、算法题解、灵感和知识沉淀到一个站点里的自研系统。",
			ContentMD: projectMD,
			Tags:      []string{"go", "react", "postgresql"},
			Meta:      json.RawMessage(`{"stack":"Go 1.22 + chi + pgx + PostgreSQL 16 + React 18 + Vite","repo_url":"https://github.com/fenghuangyuan1412/kb-blog","highlights":["单表承载四种内容","笔记支持无限层级父子","PostgreSQL 全文检索"]}`),
		}},
		{SaveInput: service.SaveInput{
			Type:      model.TypeAlgorithm,
			Slug:      "binary-search",
			Title:     "二分查找",
			Summary:   "在有序数组里找目标值，每比一次就把范围砍一半。",
			ContentMD: binarySearchMD,
			Tags:      []string{"算法", "二分"},
			Meta:      json.RawMessage(`{"difficulty":"easy","source":"LeetCode 35","tags":["数组","边界"]}`),
		}},
		{SaveInput: service.SaveInput{
			Type:      model.TypeIdea,
			Slug:      "test-minded-reading",
			Title:     "用测试的眼光读代码",
			Summary:   "读代码时先问“这段要怎么测”，比先问“这段在干什么”更快找到入口。",
			ContentMD: ideaMD,
			Tags:      []string{"学习方法", "测试"},
			Meta:      json.RawMessage(`{}`),
		}},
		{SaveInput: service.SaveInput{
			Type:      model.TypeNote,
			Slug:      "testing",
			Title:     "软件测试",
			Summary:   "测试学习主线：用例设计、缺陷管理、测试流程。",
			ContentMD: testingIndexMD,
			Tags:      []string{"测试", "学习"},
			Meta:      json.RawMessage(`{}`),
		}},
		{SaveInput: service.SaveInput{
			Type:      model.TypeNote,
			Slug:      "black-box",
			Title:     "黑盒用例设计",
			Summary:   "不看内部实现，只按输入输出设计用例的几种常用方法。",
			ContentMD: blackBoxMD,
			Tags:      []string{"测试", "用例设计"},
			Meta:      json.RawMessage(`{}`),
		}, Parent: "testing"},
		{SaveInput: service.SaveInput{
			Type:      model.TypeNote,
			Slug:      "equivalence-partition",
			Title:     "等价类划分",
			Summary:   "把输入域切成若干等价类，每类只测一个代表值。",
			ContentMD: equivalenceMD,
			Tags:      []string{"测试", "用例设计"},
			Meta:      json.RawMessage(`{}`),
			SortOrder: 1,
		}, Parent: "black-box"},
		{SaveInput: service.SaveInput{
			Type:      model.TypeNote,
			Slug:      "boundary-value",
			Title:     "边界值分析",
			Summary:   "缺陷最爱待在边界上，取上下边界和刚好越界的值。",
			ContentMD: boundaryMD,
			Tags:      []string{"测试", "用例设计"},
			Meta:      json.RawMessage(`{}`),
			SortOrder: 2,
		}, Parent: "black-box"},
		{SaveInput: service.SaveInput{
			Type:      model.TypeNote,
			Slug:      "white-box",
			Title:     "白盒测试",
			Summary:   "按代码结构设计用例，用覆盖率判断测够没有。",
			ContentMD: whiteBoxMD,
			Tags:      []string{"测试", "覆盖率"},
			Meta:      json.RawMessage(`{}`),
			SortOrder: 2,
		}, Parent: "testing"},
		{SaveInput: service.SaveInput{
			Type:      model.TypeNote,
			Slug:      "coverage-criteria",
			Title:     "语句覆盖与判定覆盖",
			Summary:   "覆盖准则的强弱关系；这篇留成草稿，用来验证管理员能看到 draft。",
			ContentMD: coverageMD,
			Tags:      []string{"测试", "覆盖率"},
			Meta:      json.RawMessage(`{}`),
			Status:    model.StatusDraft,
			SortOrder: 1,
		}, Parent: "white-box"},
	}
}

const projectMD = `一个自用站点：访客能读、能搜、能按标签翻，只有我自己能写。

## 为什么自建

- 学习记录散在本地 txt 和笔记软件里，检索不动。
- 想顺手把 Go 后端和 React 前端各练一遍，做完要有真东西留下来。

## 技术选择

后端 Go 1.22 + chi + pgx + PostgreSQL 16，前端 React 18 + Vite + TypeScript + Tailwind。
内容全塞进一张 ` + "`documents`" + ` 表，用 type 区分项目/算法/思路/笔记，笔记靠 parent_id 支持无限层级。

## 当前进度

Day 1 后端接口全部 curl 得通，Day 2 做前台阅读，Day 3 做后台编辑和部署。
`

const binarySearchMD = `给定升序数组和目标值，返回下标；找不到返回 -1。要求 O(log n)。

## 思路

维护左右边界，每次取中点比较，把搜索范围砍一半。难点全在边界，` + "`<`" + ` 写成 ` + "`<=`" + ` 就漏掉最后一个元素。

` + binarySearchCode + `
## 测试要点

空数组、单元素命中/不命中、目标是最小值/最大值、比最小还小/比最大还大、有重复元素。
`

// Go 源码片段里有反引号会截断原始字符串，这段用普通字符串按行拼。
const binarySearchCode = "```go\n" +
	"func search(arr []int, target int) int {\n" +
	"    left, right := 0, len(arr)-1\n" +
	"    for left <= right {\n" +
	"        mid := left + (right-left)/2 // 防加法越界\n" +
	"        switch {\n" +
	"        case arr[mid] == target:\n" +
	"            return mid\n" +
	"        case arr[mid] > target:\n" +
	"            right = mid - 1\n" +
	"        default:\n" +
	"            left = mid + 1\n" +
	"        }\n" +
	"    }\n" +
	"    return -1\n" +
	"}\n" +
	"```\n\n"

const ideaMD = `拿到一段看不懂的代码，别从头逐行读。

先问三个测试问题：

1. 这段代码的输入有哪些类型？（参数、状态、外部数据）
2. 出错会体现在哪？（返回值、副作用、日志）
3. 我要证明它工作，最少需要几条用例？

这么一问，入口和关键分支自己就浮出来了，读代码变成"找可断言的地方"。
正好也是测试工程师的日常思维，转行期一举两得。
`

const testingIndexMD = `转测试方向的自学主线，按"会设计用例 → 会定位缺陷 → 会走流程"三块推进。

- 黑盒用例设计：等价类、边界值、判定表、场景法。
- 白盒：语句/判定/条件覆盖。
- 缺陷管理：复现步骤、期望/实际、严重级别与优先级。

子节点是每块的具体方法，草稿状态的暂时只有我自己看得到。
`

const blackBoxMD = `把程序当黑盒子，只管"给什么输入、该出什么结果"，不关心内部怎么写。

常用方法：

| 方法 | 什么时候用 | 特点 |
| --- | --- | --- |
| 等价类划分 | 输入域能分类 | 用例数最少，覆盖面靠分类是否合理 |
| 边界值分析 | 有数值/长度范围 | 和等价类搭配，专治差一个 |
| 判定表 | 多个条件组合决定结果 | 能把漏掉的组合逼出来 |
| 因果图 | 输入之间互相约束 | 画完可转成判定表 |
| 场景法 | 有业务流程 | 按用户路径设计，接近真实使用 |
`

const equivalenceMD = `等价类划分把程序的输入域切成若干份，每一份里任意一个值都能代表整份。

## 怎么分

- 有效等价类：满足规格输入的集合。
- 无效等价类：不满足的部分，越界、格式错、空值都算。
- 一个条件能拆多份就再细分，比如"1-100 的整数"可拆成 <1、1-100、>100。

## 例子：注册页手机号校验

规格：11 位数字，以 13-19 开头。

| 等价类 | 代表值 | 预期 |
| --- | --- | --- |
| 有效 | 13800000000 | 通过 |
| 位数不足 | 1380000000 | 拒绝并提示 |
| 非数字 | 1380000000a | 拒绝并提示 |
| 号段不存在 | 12800000000 | 拒绝并提示 |

## 易错点

只测有效类，上线后用户乱填直接崩；无效类加多了用例又涨得飞快，先按风险挑。
`

const boundaryMD = `边界值分析和等价类是搭档：等价类告诉你分几份，边界值告诉你每份里挑哪个值最划算。

## 取值口诀

上点、离点、内点：

- 上点：边界本身（1、100）。
- 离点：刚好越界的那个（0、101）。
- 内点：范围内随便一个代表值（50）。

## 例子：年龄输入 18-60

17、18、19、59、60、61 六个值，能覆盖到绝大多数"差一个"的 bug。

## 为什么有效

开发写条件时最容易把 ` + "`<`" + ` 和 ` + "`<=`" + ` 写反，边界值正好把这种错误照出来。
`

const whiteBoxMD = `白盒测试按代码内部结构设计用例，覆盖率是"测够了没有"的量尺。

要看的对象：语句、判定、条件、路径。写用例前先画出控制流图，再按准则凑覆盖，比凭感觉快也全。

代价是：覆盖率高不等于没问题，需求漏了、逻辑写反了，代码内部照样自洽。所以白盒补充黑盒，不是替代。
`

const coverageMD = `覆盖准则从弱到强大致是：语句覆盖 < 判定覆盖 < 条件覆盖 < 判定/条件覆盖 < 条件组合覆盖 < 路径覆盖。

- 语句覆盖：每行至少走一次。最弱，连 ` + "`if`" + ` 的假分支都可能漏。
- 判定覆盖：每个判定的真/假各走一次。
- 条件组合覆盖：每个判定里各条件的取值组合都出现。
- 路径覆盖：可行路径全走一遍，代价最高。

## 用法

覆盖率是"没测到什么"的证据，不是"测好了"的结论。100% 语句覆盖照样可能漏需求。
`
