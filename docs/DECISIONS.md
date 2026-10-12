# DECISIONS —— 技术决策记录

格式：背景 / 选项 / 结论 / 理由。新决策往下追加，不删旧的（推翻旧决策时补一条新的并标注取代关系）。

---

## D1 四类内容用一张 `documents` 表，而不是四张表

- 背景：project / algorithm / idea / note 字段差异不大，主要差别在专属元信息。
- 选项：(a) 单表 + `type` + `meta JSONB`；(b) 四张表各带专有列；(c) 公共表 + 类型子表。
- 结论：(a) 单表。
- 理由：列表、搜索、标签聚合、后台编辑器都能共用一套 SQL 和一套接口；(b) 会让"跨类型搜索"变成四路 UNION，(c) 的 JOIN 复杂度对一个单人三天冲刺的项目不划算。专属字段用 `meta` 约定（见 DATA_MODEL.md）而不是列。

## D2 本机不用 Docker，PostgreSQL 用便携版安装

- 背景：主提示词写的是"docker-compose 起 Postgres"，但本机没有 Docker Desktop，装它需要 WSL2 + 虚拟化 + 重启。
- 选项：(a) 装 Docker Desktop；(b) EDB 官方 .msi 安装向导；(c) 官方 binaries.zip 解压便携版，脚本手动起停。
- 结论：(c)。Go 工具链同样用官方 zip 解压到 `E:\programme\go`。
- 理由：不动注册表、不装系统服务、不占 C 盘（C 盘只剩 16 GB），删目录即卸载；对应用代码零影响（连接串一样）。`docker-compose.yml` 仍保留在仓库里作为部署与 CI 的参考。
- 影响：`CLAUDE.md` 的「本机环境说明」一节记录了这一例外。

## D3 本机用 `scripts/*.ps1` 承担 `make` 的职责

- 背景：本机没有 make / winget / scoop，主提示词要求"Makefile 统一命令"。
- 选项：(a) 额外下载 standalone make.exe；(b) PowerShell 脚本 + 保留 Makefile。
- 结论：(b)。Makefile 的目标与 ps1 脚本一一对应，CI 和 Linux 用 make，本机用 ps1。
- 理由：多引一个二进制依赖只为跑 make 不值当；命令名保持一致，切换环境时肌肉记忆不变。

## D4 本地数据库端口用 5433 而不是 5432

- 背景：本机装过 SQL Server / MySQL 等，5432 可能被占或将来被占。
- 结论：便携实例监听 127.0.0.1:5433，写在 `.env` 的 `PGPORT` / `DATABASE_URL` 里。
- 理由：与可能存在的其他 PG 实例隔离；生产 Neon 用标准端口，端口只在本机配置里出现，不进代码。

## D5 Go 严格锁 1.22，依赖版本随之降级

- 背景：主提示词锁定 Go 1.22。实测 chi v5.3+、pgx v5.8+、goose v3.24.2+ 的 `go` 指令已是 1.23 / 1.24，在 1.22 工具链下无法编译。
- 选项：(a) 升到当前稳定版 Go（偏离锁定栈）；(b) 保持 Go 1.22，把依赖钉在支持 1.22 的最后版本。
- 结论：(b)。钉版：chi v5.2.2、pgx v5.7.4、goose v3.24.0、golang-jwt/v5 v5.2.2、x/crypto v0.31.0。
- 理由：语言版本是用户明确锁定的东西，不该由 Agent 擅自换；依赖这些版本没有已知阻塞问题。
- 后续：若哪天要升到 Go 1.24，同时把上面五个依赖提到最新，属技术栈变更，先议。

## D6 `GOTOOLCHAIN=local` + GOPROXY 用 goproxy.cn、校验走 sum.golang.google.cn

- 背景：本机直连 github.com 需要代理，`proxy.golang.org` / `sum.golang.org` 拉不动；Go 默认 `GOTOOLCHAIN=auto` 会在依赖要求更高版本时自动下工具链（会破坏 D5 的锁定）。
- 结论：`GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=sum.golang.google.cn`、`GOTOOLCHAIN=local`，`go env -w` 持久化在用户配置里。
- 理由：保持校验链（不是关 sumdb）又能直连下载；`local` 保证不会偷偷下第二个 Go。
- 风险：`go.sum` 仍进仓库，任何版本变动都能被发现。

## D7 goose 以库的形式集成进 `backend/cmd/migrate`，不装独立 CLI

- 背景：迁移工具要么用 CLI 二进制，要么嵌进项目。
- 选项：(a) `go run github.com/pressly/goose/v3/cmd/goose@vX`；(b) 自己写 30 行的 `cmd/migrate` 调 goose 库。
- 结论：(b)。
- 理由：不依赖每次联网解析工具版本；`go build ./...` 会把迁移入口一起编译和检查；CI 里 `./cmd/migrate up` 一条命令即可。

## D8 单管理员：凭据放 `.env`，密码只存 bcrypt 哈希，不建 users 表

- 背景：明确不做注册和多用户，但仍需要一个能校验的密码来源。
- 选项：(a) 建 `users` 表单行；(b) `.env` 里存 bcrypt 哈希；(c) `.env` 里存明文并加两条比较分支。
- 结论：(b)，用 `cmd/kbhash` 生成哈希，登录时 `bcrypt.CompareHashAndPassword`。
- 理由：单用户建表属于为不存在的需求做抽象；(c) 的"明文/哈希两条分支"是多余的错误面，也更容易把明文写进别的地方。

## D9 JWT：HS256 + httpOnly Cookie，SameSite=Lax，7 天有效

- 背景：只有管理员用，前端与后端本地不同源（5173 / 8080），线上同域或需配 CORS。
- 结论：令牌只放 Cookie（`kb_token`），不给前端读；`SameSite=Lax`；生产 `Secure=true`；CORS 白名单来自 `ALLOWED_ORIGINS` 且 `credentials=true`。
- 理由：避免 XSS 偷 token；写接口都是 POST/PUT/DELETE，Lax 下跨站表单提交不会带 Cookie，因此 Day 3 前不额外做 CSRF token。
- 遗留：若 Day 3 前端与后端不同源部署（Vercel → Fly.io），Cookie 需要 `SameSite=None; Secure`，属浏览器行为约束，届时记录。

## D10 全文搜索用 `tsvector('simple')`，接受中文不分词

- 背景：主提示词要求省掉 ES，用 `tsvector`。中文按空格/标点切词，`simple` 配置不切中文词。
- 结论：Day 1 用 `simple`，查询把连续文本切成词项 AND 组合，末词加 `:*` 前缀匹配；中文短语命中依赖 title/summary 与整段文本包含。
- 理由：`simple` 无需扩展、无需重编译数据库；zhparser / pg_trgm 需装扩展，Neon 免费层未必支持。
- 验证方式：Day 3 用真实笔记测命中率，不达标再议（属技术栈变更）。

## D11 笔记树：一次查出全部 note，在内存组装，不做递归 SQL

- 背景：树查询可以递归 CTE，也可以全量取回组装。
- 结论：repo 一条 SQL 取回全部 note 的五个字段，service 内用 map 组装 children。
- 理由：笔记树接口明确只要 id/slug/title/parent_id/sort_order，数据量（千级）下全量比 N 次递归查询便宜且代码短；返回体小。
- 边界：若笔记数量到万级，改递归 CTE 只取某子树 —— 那时再改。

## D12 `status='draft'` 的可见性由认证中间件决定，不在 handler 里判断

- 背景：draft 只能后台看到。
- 结论：中间件把「是否已登录」写进 request context，repo 查询按该标志加 `status` 条件。
- 理由：避免每个 handler 各写一遍；符合"业务规则放 service，不在 handler"的分层约定。

## D13 提交前不做前端 lint 自动化（Day 1 无前端）

- 背景：`project-dev` Skill 要求每个阶段自测。
- 结论：Day 1 自测 = `go vet` + `go test ./...` + `scripts/smoke.ps1`；前端 lint 留到 Day 2 与 `npm run build` 一起做。
- 理由：没有代码就没有可验证项，不假装完成。

## D14 本机拉 GitHub 需走代理，不写进仓库配置

- 背景：本机 `github.com` 直连失败，代理 `127.0.0.1:7890` 可用；这是用户机器网络环境，不是项目属性。
- 结论：push 时用 `git -c http.proxy=http://127.0.0.1:7890 push ...`，不改仓库或全局 git 配置。
- 理由：把个人网络环境写进仓库配置会污染他人/CI 的 clone。

## D15 .env 里含 `$` 的值一律用单引号（bcrypt 哈希踩过坑）

- 背景：Day 1 实测 `ADMIN_PASSWORD_HASH=$2a$10$...` 被 godotenv 解析成 51 个字符的残值，
  登录直接 500（`crypto/bcrypt: hashedSecret too short`）。用 PowerShell 的 env.ps1 起服务时反而正常，
  因为那是原样设置环境变量、不做插值。
- 选项：(a) 换掉 godotenv 用标准库自己解析 .env；(b) 约定值里含 `$` 时用单引号包裹。
- 结论：(b)。`.env.example`、`scripts/hashpw.ps1` 的输出都按单引号形式给。
- 理由：godotenv 是社区标准做法，问题只在写法约定；单引号是它明确的"不做插值"语法，成本最低。
- 验证：`curl POST /api/auth/login` 从 500 变 200（见 Day 1 冒烟输出）。

## D16 单测连真实开发库，包之间必须串行

- 背景：全文检索、数组列、生成列这些行为 mock 不出来，仓库层和业务层测试只能连真库；
  两者都靠 `TRUNCATE documents` 保证起点干净。
- 选项：(a) 每包独立 schema 或独立数据库；(b) 共用 kb 开发库，用 `go test -p 1 ./...` 串行。
- 结论：(b)，并在两个 setup 里写死守卫：库名不是 kb 直接 skip，CI 上设 `CI=true` 也 skip。
- 理由：单人项目，多建库的成本大于收益；串行跑全套 1 秒内结束，没有性能压力。
- 验证：不带 `-p 1` 时 `TestList_DraftVisibility` 等三条因互相清数据而失败，加上后全绿。

## D17 seed 默认拒绝非 kb 库

- 背景：`seed.go` 会先 `TRUNCATE documents RESTART IDENTITY CASCADE`，误指向别的库就是删数据。
- 选项：(a) 只插不删（重复跑会 slug 冲突）；(b) 保持幂等清空，但库名不是 kb 就退出，`-force` 才放行。
- 结论：(b)。样例数据要能反复重置，风险用一个显式开关挡住。
- 验证：`go run ./scripts` 在 kb 上连跑两次结果一致（9 篇、草稿 1 篇、三层树）。
