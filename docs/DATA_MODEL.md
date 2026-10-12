# DATA MODEL —— documents 单表

四类内容（project / algorithm / idea / note）共用一张表，靠 `type` 分流，
类型专属字段进 `meta JSONB`，笔记层级靠 `parent_id` 自引用。

## 1. 建表 SQL（迁移文件 `backend/migrations/000001_create_documents.sql` 与此一致）

```sql
CREATE TABLE documents (
  id         BIGSERIAL PRIMARY KEY,
  type       TEXT NOT NULL CHECK (type IN ('project','algorithm','idea','note')),
  parent_id  BIGINT REFERENCES documents(id) ON DELETE CASCADE,  -- 只有 note 用，实现嵌套
  slug       TEXT NOT NULL UNIQUE,
  title      TEXT NOT NULL,
  summary    TEXT NOT NULL DEFAULT '',
  content_md TEXT NOT NULL DEFAULT '',
  tags       TEXT[] NOT NULL DEFAULT '{}',
  meta       JSONB NOT NULL DEFAULT '{}',
  status     TEXT NOT NULL DEFAULT 'published',
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE documents ADD COLUMN search_vec tsvector
  GENERATED ALWAYS AS (
    to_tsvector('simple', title || ' ' || summary || ' ' || content_md)
  ) STORED;

CREATE INDEX idx_doc_type   ON documents (type, sort_order, created_at DESC);
CREATE INDEX idx_doc_parent ON documents (parent_id);
CREATE INDEX idx_doc_search ON documents USING GIN (search_vec);
```

## 2. 字段含义

| 字段 | 类型 | 含义 | 约束 / 备注 |
|---|---|---|---|
| `id` | BIGSERIAL | 主键 | 写接口用 `:id` 定位 |
| `type` | TEXT | 分类 | CHECK 限定四个值，非法值返回 400 |
| `parent_id` | BIGINT | 父笔记 | 仅 `note` 使用；NULL 表示位于树顶层；删除父节点级联删除子树 |
| `slug` | TEXT | URL 标识 | **全表唯一**，冲突返回 409；创建后不随意改（改了旧链接失效） |
| `title` | TEXT | 标题 | 必填 |
| `summary` | TEXT | 摘要 | 列表页展示，同时进全文索引 |
| `content_md` | TEXT | Markdown 正文 | 前端渲染，后端不解析 |
| `tags` | TEXT[] | 标签 | 小写存储，repo 层写入前统一 `strings.ToLower` + 去重 |
| `meta` | JSONB | 类型专属字段 | 见下表；不做 schema 校验以外的强约束 |
| `status` | TEXT | 发布状态 | `published` / `draft`；`draft` 只有认证请求可见 |
| `sort_order` | INT | 同级排序 | 升序；`move` 接口可改 |
| `created_at` / `updated_at` | TIMESTAMPTZ | 时间 | `updated_at` 由 repo 层在 UPDATE 时显式写 `now()` |
| `search_vec` | tsvector | 生成列 | STORED，由 title+summary+content_md 生成，`simple` 配置；**只读**，禁止手改 |

## 3. meta 约定

| type | meta 结构 |
|---|---|
| project | `{ "repo": string, "demo": string, "tech": string[], "status": "doing"\|"done", "started_at": string, "ended_at": string }` |
| algorithm | `{ "lang": string, "time_complexity": string, "space_complexity": string, "source": string }` |
| idea | `{ "mood": string, "related_ids": number[] }` |
| note | `{}` |

`meta` 允许缺字段（读取时给零值），但不许出现上面之外的野字段被前端依赖。

## 4. 索引与查询对应关系

| 索引 | 服务的查询 |
|---|---|
| `documents_slug_key`（UNIQUE 自带） | `GET /api/documents/:slug` |
| `idx_doc_type` | `GET /api/documents?type=&page=` 的过滤 + 排序 |
| `idx_doc_parent` | `GET /api/notes/tree`、`children_count` 计数 |
| `idx_doc_search`（GIN） | `q=` / `GET /api/search` 的 `@@ to_tsquery('simple', ...)` |
| tags 暂无索引 | 数据量到万级再考虑 GIN on `tags`，现在不提前优化 |

## 5. 层级完整性规则（在 service 层实现，不靠数据库）

1. 只有 `type='note'` 允许 `parent_id` 非空；其他类型若带 `parent_id` → 400
2. 父节点必须存在且 `type='note'` → 否则 400 `parent_not_found`
3. `move` 时不许把节点挂到自己或自己的后代下面（会成环）→ 400 `cycle_detected`
4. 树的读取用一次查询取全部 note，在内存里组装（避免递归 N 次查询）

## 6. 迁移与回滚

- 工具：goose（`-- +goose Up` / `-- +goose Down` 成对写）
- 迁移文件进 Git，禁止手改表结构
- Day 1 的 down 迁移就是 `DROP TABLE documents CASCADE;`
- 上线后的迁移必须先在 Neon 的分支库演练，且带可执行的 down
