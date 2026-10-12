---
name: version-control
description: Git 与版本管理规范。任何 commit / 分支 / PR 操作前必读。
---

## 分支
- 主干 `main`，永远可运行、可部署
- 功能分支：`feat/<scope>-<短描述>`，如 `feat/notes-tree`
- 修复分支：`fix/<scope>-<短描述>`
- 分支存活不超过 1 天，做完立刻合回 main

## Commit（Conventional Commits）
格式：`<type>(<scope>): <中文描述>`

type：feat | fix | refactor | docs | test | chore | style | perf
scope：backend | frontend | db | auth | notes | projects | ci | docs

好例子：
  feat(backend): 新增 documents 列表分页查询
  fix(frontend): 修复笔记树折叠状态丢失
  docs(api): 补充 /api/search 的 type 参数说明

坏例子：
  update          ← 没说什么
  修复bug         ← 没有 type 和 scope
  WIP             ← 不许推到远端

## 粒度
- 一个 commit 只做一件事，能独立回滚
- 代码 + 对应文档 + 迁移文件 放同一个 commit
- 禁止一个 commit 同时改后端和前端（除非是接口契约同步变更）

## 每次 commit 前
- [ ] `git status` 看过，没有意外文件
- [ ] 没有 `.env`、密钥、node_modules、编译产物被加进来
- [ ] 本地能跑通
- [ ] message 符合格式

## 合并
- 合回 main 前先 `git pull --rebase origin main` 解决冲突
- 合并后立刻删掉功能分支
- 每个里程碑打 tag：`v0.1.0-day1`

## 绝对禁止
- `git push --force` 到 main
- `git add .` 不看内容
- 提交注释掉的代码和调试 console.log

## 本机补充
- 提交前用 `git status --short` 逐个确认文件；`git add` 只按文件名加，不用 `git add .`
- 本仓库当前直接在 main 上小步提交（单人单日冲刺），里程碑打 tag；超过一天的工作才开功能分支
