# API —— 接口契约

Base URL：`http://localhost:8080`（生产环境同源由前端配置注入）
请求 / 响应体一律 `application/json; charset=utf-8`。
路径与字段名是契约，改动必须同步本文件并另开 commit。

## 0. 通用约定

### 成功

直接返回数据体，不套 `data` 外壳。

### 失败

```json
{ "error": { "code": "slug_conflict", "message": "slug 已存在：hello-world" } }
```

| HTTP | code | 触发场景 |
|---|---|---|
| 400 | `bad_request` | JSON 解析失败、必填缺失、`type` 非法、非 note 带 parent_id |
| 400 | `parent_not_found` | 指定的父节点不存在或不是 note |
| 400 | `cycle_detected` | move 会把节点挂到自己或后代下 |
| 401 | `unauthorized` | 无 Cookie / Cookie 过期 / 签名不合法 |
| 403 | `wrong_credentials` | 登录邮箱或密码错误 |
| 404 | `not_found` | slug 或 id 不存在 |
| 409 | `slug_conflict` | 创建或更新时 slug 撞车 |
| 500 | `internal_error` | 服务端未预期错误（响应里不含 SQL 和堆栈） |

### 可见性

`status='draft'` 的文档只有带有效 JWT Cookie 的请求才能读到；
公开列表和公开详情对 draft 一律 404。

### 分页与排序

`page` 从 1 开始（默认 1），`size` 默认 20、上限 100。
排序固定：`sort_order ASC, created_at DESC`。

### 文档对象字段

列表项（`DocumentListItem`）：

```json
{
  "id": 1, "type": "project", "slug": "kb-blog", "title": "个人知识库",
  "summary": "四类内容统一存储", "tags": ["go", "react"],
  "status": "published", "created_at": "2026-10-13T01:00:00Z",
  "updated_at": "2026-10-13T01:00:00Z"
}
```

详情（`Document`）在列表项基础上增加：`parent_id`、`sort_order`、`content_md`、`meta`、`children_count`。

## 1. 公开接口

### 1.1 列表 / 查询

```
GET /api/documents?type=&tag=&q=&page=&size=
```

| 参数 | 说明 |
|---|---|
| `type` | `project` / `algorithm` / `idea` / `note`，逗号分隔可多值（如 `type=project,idea`）；非法值 400 |
| `tag` | 单个标签，精确匹配 `tags` 数组包含 |
| `q` | 全文搜索，走 `search_vec @@ to_tsquery('simple', ...)`；多词按 AND 组合，末词加前缀匹配 |
| `page` / `size` | 见通用约定 |

draft 是否出现由登录状态决定（带有效 Cookie 即管理员视角），不提供 `include_draft` 参数，避免给匿名请求开后门。

响应 `200`：

```json
{ "items": [ /* DocumentListItem[] */ ], "total": 137, "page": 1, "size": 20 }
```

`total` 是满足过滤条件的总条数（用于分页）。空结果返回 `items: []`、`total: 0`，不是 404。

### 1.2 单篇详情

```
GET /api/documents/:slug
```

`200` → `Document`（含 `children_count`，note 的直接子笔记数量）。
`404 not_found`。

### 1.3 笔记树

```
GET /api/notes/tree
```

`200` → 嵌套数组，只含五个字段，不含正文：

```json
[
  { "id": 3, "slug": "testing", "title": "软件测试", "parent_id": null, "sort_order": 0,
    "children": [
      { "id": 7, "slug": "black-box", "title": "黑盒用例设计", "parent_id": 3, "sort_order": 0,
        "children": [
          { "id": 9, "slug": "equivalence-partition", "title": "等价类划分", "parent_id": 7, "sort_order": 0, "children": [] }
        ] }
    ] }
]
```

一次查询取全部 note，在内存组装；同级按 `sort_order ASC, title ASC`。

### 1.4 标签

```
GET /api/tags
```

`200` → 按 count 倒序：

```json
[ { "tag": "go", "count": 12 }, { "tag": "pytest", "count": 5 } ]
```

只统计已发布文档（带有效 Cookie 时含 draft，由 `authOnly` 中间件决定）。

### 1.5 搜索

```
GET /api/search?q=&type=
```

与 `GET /api/documents` 复用同一查询实现，响应结构相同；`q` 缺失或为空返回 400 `bad_request`。

## 2. 认证接口

单管理员，凭据来自 `.env` 的 `ADMIN_EMAIL` / `ADMIN_PASSWORD`（密码用 bcrypt 存哈希在 `.env`，明文只在初始化时出现）。
令牌是 HS256 JWT，写进 `kb_token` Cookie：`HttpOnly`、`SameSite=Lax`、`Path=/`、`Max-Age=604800`（7 天）、生产加 `Secure`。

### 2.1 登录

```
POST /api/auth/login
{ "email": "admin@example.com", "password": "..." }
```

`200` → `{ "email": "admin@example.com" }` + `Set-Cookie: kb_token=...`
`403 wrong_credentials`（不区分邮箱不存在还是密码错，避免枚举）。

### 2.2 登出

```
POST /api/auth/logout
```

`200` → `{ "ok": true }`，并下发同名空值 + `Max-Age=0` 的 Cookie 清除。

### 2.3 当前身份

```
GET /api/auth/me
```

`200` → `{ "email": "admin@example.com" }`；无有效 Cookie → `401 unauthorized`。

## 3. 写接口（全部需要有效 Cookie）

### 3.1 新建

```
POST /api/documents
{
  "type": "note", "slug": "equivalence-partition", "title": "等价类划分",
  "summary": "把输入域切成可代表整体的子集", "content_md": "# 正文",
  "tags": ["测试", "用例设计"], "meta": {}, "status": "published",
  "parent_id": 7, "sort_order": 0
}
```

`201` → `Document`。`409 slug_conflict`。`parent_id` 只允许 note（见 DATA_MODEL 第 5 节）。

### 3.2 更新

```
PUT /api/documents/:id
```

全量覆盖语义：请求体里出现的字段覆盖，`tags` / `meta` / `content_md` 缺省视为空值。
`type` 允许修改，但把有子节点的 note 改成其他 type 时返回 400 `bad_request`。
`200` → `Document`；`404`；`409`。

### 3.3 删除

```
DELETE /api/documents/:id
```

`200` → `{ "id": 9, "deleted": true }`。note 的删除会级联删掉整棵子树（数据库 `ON DELETE CASCADE`），
所以后台二次确认必须写明"连同 N 篇子笔记一起删除"。

### 3.4 移动 / 排序

```
POST /api/documents/:id/move
{ "parent_id": 3, "sort_order": 2 }
```

`parent_id` 为 `null` 表示移到树顶层。`200` → `Document`；
`404`（自身或父不存在）；`400 cycle_detected`。

## 4. 运维接口

```
GET /healthz   → 200 { "status": "ok", "db": "ok", "version": "0.1.0" }
                 DB 连接失败时 → 503 { "error": { "code": "db_unavailable", "message": "..." } }
```
