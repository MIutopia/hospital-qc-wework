# 住院病例质控企业微信推送系统（hospital-qc-wework）

> 本仓库通过 GitHub 进行项目管理：**版本控制 + Pull Request 协作 + Issue 任务看板**。
> 仓库已正式命名为 `hospital-qc-wework`（旧名 build-project 自动重定向，无破坏）。

## 项目简介

某医院住院部每日产生大量住院病例。质控科通过本系统对病例进行规则化质量审核，发现问题后精准推送给责任医生，医生在企业微信中点击消息即可查看详细质控报告并确认整改。

核心流程：`HIS 数据库 → 数据同步 → 质控规则引擎 → 企业微信消息推送 → 医生 H5 页面确认`

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.22+ / Gin / go-mssqldb / expr-lang/expr / robfig/cron |
| 数据库 | SQL Server 2014 |
| 缓存 | Memurai (Windows Redis) / 进程内 MemoryCache |
| 前端 | Vue 3 + Vant 4 + Vite |
| 部署 | NSSM Windows Service + Nginx for Windows |

## 项目结构

```
hospital-qc-wework/
├── backend/          # Go 后端服务
├── frontend/         # Vue 3 H5 前端
├── sql/              # 数据库脚本（DDL + 初始数据 + 测试数据）
├── deploy/           # 部署配置（nginx.conf + build.bat）
└── docs/             # 项目文档
```

## 仓库约定

- 默认分支：`main`，**受保护**，必须通过 Pull Request 合并，不能直接 push。
- 开发分支：`feature/*`（新功能）、`fix/*`（缺陷修复）、`docs/*`（文档）。
- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)：
  `feat:` / `fix:` / `docs:` / `chore:` / `refactor:` / `test:`。
- 任务跟踪：GitHub Issues + Project 看板，所有工作都应有关联 Issue。

## 快速开始

```bash
# 后端启动（开发）
cd backend
go mod tidy
go run cmd/server/main.go --config config.yaml

# 前端启动（开发）
cd frontend
npm install
npm run dev
```

## 文档

- [项目模块分解与交接文档](docs/项目模块分解与交接文档.md) — 项目管理的总控文档，开发人员必读
- [API 接口文档](docs/api.md) — 全部接口定义
- [第1周开发任务卡](docs/第1周开发任务卡.md) — M1 开发阶段的任务分解
- [BUILD_PLAN.md](BUILD_PLAN.md) — 构建方案
