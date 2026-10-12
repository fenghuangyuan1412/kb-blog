# PLAN —— 里程碑与每日 checklist

状态标记：`[x]` 已完成并验证，`[ ]` 未开始，`[~]` 进行中。

## Day 1 —— 骨架 + 后端端到端（可 curl）

验收：`curl` 能跑通 登录 → 建嵌套笔记 → 树查询 → 全文搜索 → 改 → 删。

- [x] 本机工具链：便携版 Go 1.22.12（`E:\programme\go`）+ PostgreSQL 16.6（`E:\programme\pgsql`，端口 5433）
- [x] `git init`（main）+ `.gitignore` + `.env.example` + `.gitattributes`
- [x] `CLAUDE.md` 主提示词
- [x] `.claude/skills/{frontend,version-control,project-dev}/SKILL.md`
- [x] `docs/PRD.md`
- [x] `docs/API.md`
- [x] `docs/DATA_MODEL.md`
- [x] `docs/DECISIONS.md`
- [x] `docs/PLAN.md`（本文件）
- [x] `scripts/{env,pg-init,pg-start,pg-stop,db-create,migrate,run,seed,test,smoke,hashpw}.ps1` + `Makefile` + `docker-compose.yml`
- [x] goose 迁移：`documents` 表 + `search_vec` 生成列 + 3 个索引
- [x] 后端分层骨架：config / db / model / repo / service / handler / middleware / render + `/healthz`
- [x] 读链路：列表（type/tag/q/page/size）、详情（含 children_count）、笔记树、标签、搜索
- [x] 认证：登录 / 登出 / me（JWT HS256 + httpOnly Cookie）
- [x] 写链路：create / update / delete / move，slug 冲突 409
- [x] repo / service / middleware 单元测试（`scripts/test.ps1`，即 `go test -p 1 ./...`；两个测试包共用 kb 库，必须串行）
- [x] `backend/scripts/seed.go`：四类内容 + 三层嵌套笔记 + 1 篇草稿（幂等，默认只允许 kb 开发库）
- [x] `scripts/smoke.ps1` 冒烟：38 项全绿，0 失败，输出见 Day 1 简报
- [~] tag `v0.1.0-day1` + 推送公开仓库

## Day 2 —— 前端与阅读体验

验收：浏览器里能完整浏览四类内容，搜索有结果，笔记树能点开嵌套层级。

- [ ] Vite + React 18 + TS + Tailwind + Router v6 脚手架
- [ ] `src/api/`：类型定义（与 docs/API.md 一一对应）+ fetch 封装 + `ApiError`
- [ ] TanStack Query 配置与 `hooks/`（documents / document / tree / tags / search）
- [ ] 全局布局：导航、页脚、深色模式、375px 不破版
- [ ] 首页 `/`
- [ ] `/projects` + `/projects/:slug`
- [ ] `/algorithms` + `/algorithms/:slug`
- [ ] `/ideas` + `/ideas/:slug`
- [ ] `<Markdown />` 组件（remark-gfm + rehype-highlight + remark-toc）
- [ ] `/notes` 左侧可折叠树 + 右侧正文，`/notes/:slug` 定位
- [ ] `/tags/:tag`、`/search?q=`
- [ ] loading / empty / error 三态组件
- [ ] `npm run build` + `npm run lint` 通过并 commit

## Day 3 —— 后台 + 上线

验收：线上可访问，能从后台发一篇嵌套笔记并立刻在前台看到。

- [ ] `/admin/login` 登录页（Cookie 跨域配置验证）
- [ ] `/admin/documents` 列表 + type/status 筛选 + 删除确认（写明级联子树数量）
- [ ] `/admin/documents/:id` 编辑器：左 textarea 右实时预览、parent_id 选择器、tags/status/meta 表单
- [ ] 响应式打磨、404 页、SEO meta（title / description / canonical / og）
- [ ] `backend/Dockerfile` + `frontend` 构建产物
- [ ] GitHub Actions：lint + build + test
- [ ] 部署：后端 Fly.io、前端 Vercel、数据库 Neon + 线上迁移
- [ ] seed 真实内容（把现有学习记录搬进来）
- [ ] README 完整（本地起停、部署、常见问题）
- [ ] 复盘写进 `docs/DECISIONS.md`，tag `v1.0.0`

## 遗留问题（冲刺内不修，必须记录）

| 问题 | 影响 | 计划 |
|---|---|---|
| 中文全文搜索用 `simple` 配置，不分词 | 搜「等价类」能命中，搜「测试方法」这类组合可能漏 | Day 3 用真实内容实测命中率；不行再评估 zhparser（属技术栈变更，需先议） |
| `tags` 无索引 | 万级数据后过滤变慢 | 数据量到了再加 GIN |
| 删除笔记级联删整棵子树，无回收站 | 误删不可恢复 | Day 3 后台二次确认必须显示子树数量；长期考虑软删 |
