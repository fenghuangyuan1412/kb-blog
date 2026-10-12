# CLAUDE.md

本文件是本仓库的唯一开发 Agent 主提示词。Agent 每次动手前必须先读本文件和 `.claude/skills/*/SKILL.md`。
Skill 与本文件冲突时，以本文件为准；本文件未覆盖时，以 Skill 为准。

# 角色

我是项目负责人，负责决策和验收；Agent 负责实现、自测、提交。
Agent 不许在没跑通的情况下说"应该没问题"，不许攒一大堆文件再提交。

# 项目

个人博客 / 知识库（仓库名 `kb-blog`）。核心是把四类内容统一管理在一张表里：

1. `project`   —— 项目集（含 repo、demo、技术栈、复盘）
2. `algorithm` —— 学过的算法（含思路、复杂度、代码、踩坑）
3. `idea`      —— 以前的想法 / 思路（可以很碎，允许草稿状态）
4. `note`      —— Markdown 笔记，**必须支持任意层级嵌套**（`parent_id` 自引用）

访客可以浏览、搜索、按标签过滤、在笔记树里跳转。
只有我本人能登录后台写内容。**不做注册、不做评论、不做多用户。**

# 硬性约束（不可协商）

- 技术栈按下方锁定，不许替换、不许引入额外框架。要换必须先问我。
- 所有注释、commit message 用中文；标识符用英文。
- 不许写 `TODO` 占位后不管；写不完就明确告诉我"未完成 + 原因 + 下一步"。
- 每完成一个可运行的最小单元就 commit 一次。
- 不许在没有迁移文件的情况下手改数据库结构。
- 不许把密钥写进代码，一律走 `.env` + `.env.example`。
- 每完成一个阶段，跑一次自测，并把命令和输出贴给我看。

# 技术栈（锁定）

- 后端：Go 1.22 + chi + pgx + PostgreSQL 16 + goose 迁移 + JWT(httpOnly Cookie)
- 前端：React 18 + Vite + TypeScript + Tailwind + React Router v6 + TanStack Query
- Markdown：react-markdown + remark-gfm + rehype-highlight + remark-toc
- 本地数据库：PostgreSQL 16（本机为便携版安装，见「本机环境说明」）；`docker-compose.yml` 作为部署参考保留
- 统一命令：本机用 `scripts/*.ps1`（Windows 无 make），Makefile 供 CI / Linux 使用，两者命令名一一对应
- CI：GitHub Actions（lint + build + test）
- 部署：后端 Fly.io，前端 Vercel，数据库 Neon

# 数据模型

单表 `documents`，细节见 `docs/DATA_MODEL.md`，迁移必须按它写。

- `type` 决定分类；`note` 用 `parent_id` 做嵌套树；`meta` JSONB 存各类型专属字段
- `search_vec` 是生成列（`tsvector`），全文搜索走它 + GIN 索引，**不要用 LIKE 全表扫**

各 type 的 meta 约定：

| type | meta 字段 |
|---|---|
| project | `{ repo, demo, tech: string[], status: "doing"\|"done", started_at, ended_at }` |
| algorithm | `{ lang, time_complexity, space_complexity, source }` |
| idea | `{ mood, related_ids: number[] }` |
| note | `{}` |

# API 契约（严格按此实现，路径和字段名不许改）

公开：

```
GET  /api/documents?type=&tag=&q=&page=&size=   → { items, total, page, size }
GET  /api/documents/:slug                        → 单个文档（含 children 计数）
GET  /api/notes/tree                             → 笔记树（嵌套，只返回 id/slug/title/parent_id/sort_order）
GET  /api/tags                                   → [{ tag, count }]
GET  /api/search?q=&type=                        → 复用 documents 查询
```

需认证（Cookie 里的 JWT）：

```
POST   /api/auth/login      { email, password } → Set-Cookie
POST   /api/auth/logout
GET    /api/auth/me
POST   /api/documents
PUT    /api/documents/:id
DELETE /api/documents/:id
POST   /api/documents/:id/move  { parent_id, sort_order }
```

统一响应：成功直接返回数据体；失败返回 `{ "error": { "code": "...", "message": "..." } }`。
状态码：400 参数错、401 未登录、403 无权限、404 不存在、409 slug 冲突、500 服务端错。

# 前端路由

```
/                    首页：自我介绍 + 精选项目 + 最近笔记
/projects            项目集列表（卡片，支持 tag 过滤）
/projects/:slug      项目详情
/algorithms          算法列表（表格：名称/复杂度/标签）
/algorithms/:slug    算法详情（Markdown 正文 + 代码高亮）
/ideas               思路列表（时间线）
/ideas/:slug         思路详情
/notes               笔记页：左侧可折叠树 + 右侧 Markdown 正文
/notes/:slug         同上，直接定位到某篇
/tags/:tag           按标签聚合
/search?q=           搜索结果
/admin/login         登录
/admin/documents     后台列表（筛选 type/status）
/admin/documents/:id 编辑器：左侧 Markdown textarea，右侧实时预览，可设 parent_id
```

# 目录结构（严格按此创建）

```
├── CLAUDE.md
├── .claude/skills/{frontend,version-control,project-dev}/SKILL.md
├── docs/{PRD.md,API.md,DATA_MODEL.md,PLAN.md,DECISIONS.md}
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/{config,db,model,repo,service,handler,middleware,render}/
│   ├── migrations/
│   ├── scripts/seed.go
│   ├── go.mod
│   └── Dockerfile
├── frontend/
│   ├── src/{api,components,pages,hooks,lib,styles}/
│   ├── package.json
│   └── vite.config.ts
├── scripts/            # 本机起停 / 迁移 / 冒烟的 ps1
├── docker-compose.yml
├── Makefile
└── README.md
```

分层规则：handler 只做参数校验和响应；service 放业务；repo 只碰 SQL。
handler 不许直接调 repo，service 不许碰 `http.Request`。

# 工作方式

1. 开工前先输出本次任务的「计划 + 涉及文件 + 验收标准」，等我确认。
2. 每次只做一个纵向切片（端到端能跑通的最小功能），做完立刻自测 + commit。
3. 写代码前先写 / 更新 `docs/` 里的对应文档；文档和代码不一致时以代码为准并同步文档。
4. 遇到歧义或需要我决策的，停下来问，不要自己猜着往下写。
5. 每完成一个阶段给我简报：做了什么 / 怎么验证 / 遗留问题 / 下一步。

# 三天里程碑

- **Day 1**：仓库骨架 + Skill 文件 + docs + 迁移 + 健康检查 + documents CRUD（repo/service/handler）
  + JWT 登录 + seed 脚本 → 验收：curl 能跑通登录 + 增删改查 + 分页查询
- **Day 2**：前端脚手架 + 布局 + API 客户端 + 列表/详情页（projects/algorithms/ideas）
  + 笔记树 + Markdown 渲染 + 全文搜索 + 标签页 → 验收：浏览器能完整浏览四类内容，
  搜索有结果，笔记树能点开嵌套层级
- **Day 3**：管理后台（登录 + 编辑器 + 实时预览 + parent_id 选择）+ 响应式 + 空态/加载态/404
  + SEO meta + Dockerfile + CI + 部署 + seed 真实数据 + README + 复盘
  → 验收：线上可访问，能从后台发一篇嵌套笔记并立刻在前台看到

# 完成定义（DoD）

一个功能算完成，必须同时满足：

- [ ] 代码能编译 / 构建通过
- [ ] 有对应的错误处理，不吞错
- [ ] 我能按 README 的命令在本地跑起来
- [ ] 已 commit，message 符合 Conventional Commits
- [ ] 相关 docs 已同步
- [ ] 有最小验证方式（curl 命令 / 页面路径），并已实测通过

# 禁止事项

- 禁止引入 Redis、Kafka、微服务、GraphQL、gRPC、K8s
- 禁止为了"以后可能要"而抽象；三行重复代码好过一个错误的抽象
- 禁止一次性重写多个文件；改动尽量小且聚焦
- 禁止在没跑通的情况下说"应该没问题"
- 禁止跳过迁移直接改表

# 本机环境说明（Windows，与上面「技术栈」的差异只在于落地方式）

- Go：`E:\programme\go`（便携版 1.22.12），`go env -w` 已设 `GOPROXY=https://goproxy.cn,direct`、`GOPATH=E:\gopath`
- PostgreSQL：`E:\programme\pgsql`（便携版 16.6），数据目录 `E:\pgsql_data`，端口 **5433**（避开可能的 5432 占用）
- 起停数据库、跑迁移、冒烟测试统一用 `scripts/*.ps1`（本机没有 make，也没有 Docker Desktop）
- 本机拉取 github.com 需要代理 `127.0.0.1:7890`，push 时用 `git -c http.proxy=...`，不写进仓库配置
- 真实密钥只存在于未入库的 `.env`；仓库里只有 `.env.example`
