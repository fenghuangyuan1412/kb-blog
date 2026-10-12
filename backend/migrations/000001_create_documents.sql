-- +goose Up
-- 四类内容（project / algorithm / idea / note）共用一张表，字段含义见 docs/DATA_MODEL.md

CREATE TABLE documents (
  id         BIGSERIAL PRIMARY KEY,
  type       TEXT NOT NULL CHECK (type IN ('project','algorithm','idea','note')),
  parent_id  BIGINT REFERENCES documents(id) ON DELETE CASCADE,
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

-- 全文搜索走这个 STORED 生成列 + GIN 索引，禁止 LIKE 全表扫
ALTER TABLE documents ADD COLUMN search_vec tsvector
  GENERATED ALWAYS AS (
    to_tsvector('simple', title || ' ' || summary || ' ' || content_md)
  ) STORED;

CREATE INDEX idx_doc_type   ON documents (type, sort_order, created_at DESC);
CREATE INDEX idx_doc_parent ON documents (parent_id);
CREATE INDEX idx_doc_search ON documents USING GIN (search_vec);

-- 状态只允许两种，draft 仅在带有效 Cookie 时可见
ALTER TABLE documents ADD CONSTRAINT chk_doc_status
  CHECK (status IN ('published','draft'));

-- +goose Down
DROP TABLE IF EXISTS documents CASCADE;
