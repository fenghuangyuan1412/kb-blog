# kb-blog —— 个人知识库 / 博客

把四类内容收进一张表，公开可读、自己可写、能全文搜索。

| type | 装什么 |
|---|---|
| `project` | 项目：repo / demo / 技术栈 / 状态 / 复盘 |
| `algorithm` | 学过的算法：思路、复杂度、代码、踩坑 |
| `idea` | 碎片想法，允许草稿态 |
| `note` | Markdown 笔记，支持任意层级嵌套（`parent_id` 自引用） |

开发进度见 `docs/PLAN.md`；约束和规范见 `CLAUDE.md` 与 `.claude/skills/*/SKILL.md`。

## 技术栈

Go 1.22 + chi + pgx + PostgreSQL 16（`tsvector` 全文搜索）+ goose 迁移 + JWT(httpOnly Cookie)
前端 React 18 + Vite + TS + Tailwind + React Router v6 + TanStack Query（Day 2 起）

## 当前状态

Day 1 完成：后端可 curl 全流程（登录、增删改查、分页、标签、全文搜索、嵌套笔记树）。
Day 2/3（前端、后台、上线）见 `docs/PLAN.md`。

## 本机环境（Windows，无 Docker / make）

工具链是官方 zip 解压的便携版，全部在 C 盘之外：

| 东西 | 位置 |
|---|---|
| Go 1.22.12 | `E:\programme\go` |
| PostgreSQL 16.6 | `E:\programme\pgsql`（数据目录 `E:\pgsql_data`，端口 **5433**） |
| 仓库脚本 | `scripts/*.ps1`（与 `Makefile` 目标一一对应） |

原因与取舍记在 `docs/DECISIONS.md`（D2、D3、D4、D5、D6）。

## 快速开始

```powershell
# 0) 准备环境文件（首次）
copy .env.example .env     # 然后按需改 PG / JWT / 管理员邮箱

# 1) 起数据库（首次要先 initdb）
powershell -ExecutionPolicy Bypass -File scripts\pg-init.ps1
powershell -ExecutionPolicy Bypass -File scripts\pg-start.ps1
powershell -ExecutionPolicy Bypass -File scripts\db-create.ps1

# 2) 生成管理员密码哈希，填进 .env 的 ADMIN_PASSWORD_HASH
powershell -ExecutionPolicy Bypass -File scripts\hashpw.ps1 你的密码

# 3) 跑迁移
powershell -ExecutionPolicy Bypass -File scripts\migrate.ps1 up

# 4) 跑单测（会 TRUNCATE documents，所以放在灌数据之前）
powershell -ExecutionPolicy Bypass -File scripts\test.ps1

# 5) 灌样例数据
powershell -ExecutionPolicy Bypass -File scripts\seed.ps1

# 6) 起服务（默认 :8080）
powershell -ExecutionPolicy Bypass -File scripts\run.ps1

# 7) 另开一个终端，跑冒烟测试（登录→建三层笔记→树→搜索→改→级联删→登出）
powershell -ExecutionPolicy Bypass -File scripts\smoke.ps1
```

冒烟脚本需要管理员明文密码，只在运行时传入或交互输入，`.env` 里存的是 bcrypt 哈希：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\smoke.ps1 -Password 你的密码
```

Linux / CI 用 make：`make db-up migrate-up seed run smoke test`。

## 验证一把（服务起来之后）

```bash
curl -s localhost:8080/healthz
curl -s "localhost:8080/api/documents?type=note&size=3"
curl -s localhost:8080/api/notes/tree
curl -s "localhost:8080/api/search?q=%E7%AD%89%E4%BB%B7%E7%B1%BB"     # 等价类
curl -s -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@kb.local","password":"你的密码"}' -c /tmp/ck.txt
curl -s -b /tmp/ck.txt localhost:8080/api/auth/me
```

## 目录

```
CLAUDE.md                 Agent 主提示词（约束、契约、里程碑）
.claude/skills/           frontend / version-control / project-dev 三份规范
docs/                     PRD / API / DATA_MODEL / PLAN / DECISIONS
backend/                  Go 服务：cmd/{server,migrate,kbhash} internal/{config,db,model,repo,service,handler,middleware,render} migrations scripts
frontend/                 React 应用（Day 2 开始）
scripts/                  本机 ps1 起停 / 迁移 / 冒烟
docker-compose.yml        Postgres 的容器起法（部署与 CI 参考）
Makefile                  统一命令（CI / Linux）
```

## 不做什么

不注册、不评论、不多用户、不做统计；不引入 Redis / Kafka / 微服务 / GraphQL / gRPC / K8s；不用 ES 做搜索。详见 `docs/PRD.md` 第 4 节。
