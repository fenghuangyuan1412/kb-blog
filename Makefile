# 统一命令入口。本机 Windows 没有 make，请用 scripts/*.ps1（目标名一一对应，见 README）。
# 依赖：go 1.22、docker、curl。

GO      := go
GOFLAGS := -C backend
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
	$(GO) run $(GOFLAGS) ./cmd/migrate up

migrate-down: ## 回滚一个版本
	$(GO) run $(GOFLAGS) ./cmd/migrate down

migrate-status: ## 查看迁移状态
	$(GO) run $(GOFLAGS) ./cmd/migrate status

seed: ## 灌入四类样例数据（幂等：先清空再写）
	$(GO) run $(GOFLAGS) ./scripts/seed.go

run: ## 起后端服务
	$(GO) run $(GOFLAGS) ./cmd/server

build: ## 编译后端二进制
	$(GO) build $(GOFLAGS) -o bin/server.exe ./cmd/server
	$(GO) build $(GOFLAGS) -o bin/migrate.exe ./cmd/migrate

test: ## 跑单测
	$(GO) test $(GOFLAGS) ./...

vet: ## 静态检查
	$(GO) vet $(GOFLAGS) ./...

fmt: ## 格式化
	$(GO) fmt $(GOFLAGS) ./...

tidy: ## 整理依赖
	$(GO) mod tidy $(GOFLAGS)

smoke: ## 冒烟：登录→建嵌套笔记→树→搜索→改→删
	powershell -ExecutionPolicy Bypass -File scripts/smoke.ps1

hashpw: ## 生成管理员密码哈希：make hashpw PW=xxx
	$(GO) run $(GOFLAGS) ./cmd/kbhash "$(PW)"

frontend-dev: ## 起前端开发服务器
	npm --prefix frontend run dev

frontend-build: ## 构建前端
	npm --prefix frontend run build

clean: ## 删构建产物
	rm -f backend/bin/*.exe
