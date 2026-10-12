# 统一命令入口。本机 Windows 没有 make，请用 scripts/*.ps1（目标名一一对应，见 README）。
# 依赖：go 1.22、docker、curl。

GO      := go
# -C 必须紧跟在 go 后面，所以放在命令位而不是末尾。
GC      := -C backend
DB_URL  ?= $(shell grep -m1 '^DATABASE_URL=' .env | cut -d= -f2-)
ADDR    ?= :8080

.PHONY: help db-up db-down db-create migrate-up migrate-down migrate-status seed run \
        build test vet fmt smoke hashpw tidy frontend-dev frontend-build clean

help: ## 列出可用目标
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-18s %s\n", $$1, $$2}'

db-up: ## 起本地 Postgres（Docker）
	docker compose up -d db

db-down: ## 停本地 Postgres
	docker compose down

db-create: ## 建库（本机便携实例用；Docker 起法会自动建库）
	powershell -ExecutionPolicy Bypass -File scripts/db-create.ps1

migrate-up: ## 应用全部迁移
	$(GO) $(GC) run ./cmd/migrate up

migrate-down: ## 回滚一个版本
	$(GO) $(GC) run ./cmd/migrate down

migrate-status: ## 查看迁移状态
	$(GO) $(GC) run ./cmd/migrate status

seed: ## 灌入四类样例数据（幂等：先清空再写）
	$(GO) $(GC) run ./scripts

run: ## 起后端服务
	$(GO) $(GC) run ./cmd/server

build: ## 编译后端二进制
	$(GO) $(GC) build -o bin/server.exe ./cmd/server
	$(GO) $(GC) build -o bin/migrate.exe ./cmd/migrate

test: ## 跑单测（-p 1：repo 与 service 共用 kb 开发库，不能并发）
	$(GO) $(GC) test -p 1 -count=1 ./...

vet: ## 静态检查
	$(GO) $(GC) vet ./...

fmt: ## 格式化
	$(GO) $(GC) fmt ./...

tidy: ## 整理依赖
	$(GO) $(GC) mod tidy

smoke: ## 冒烟：登录→建嵌套笔记→树→搜索→改→删
	powershell -ExecutionPolicy Bypass -File scripts/smoke.ps1

hashpw: ## 生成管理员密码哈希：make hashpw PW=xxx
	$(GO) $(GC) run ./cmd/kbhash "$(PW)"

frontend-dev: ## 起前端开发服务器
	npm --prefix frontend run dev

frontend-build: ## 构建前端
	npm --prefix frontend run build

clean: ## 删构建产物
	rm -f backend/bin/*.exe
