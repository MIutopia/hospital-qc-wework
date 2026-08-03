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

## GitHub 协作与看板（2026-08-03 建立）

| 入口 | 链接 |
|------|------|
| **Project v2 看板（Board）** | https://github.com/users/MIutopia/projects/1 |
| Issues 列表 | https://github.com/MIutopia/hospital-qc-wework/issues |
| 里程碑「8周实施计划 (M1-M7)」 | https://github.com/MIutopia/hospital-qc-wework/milestone/1 |
| 模块标签（M1~M7） | https://github.com/MIutopia/hospital-qc-wework/labels |
| 状态标签（待开始/进行中/已完成/阻塞/外部依赖） | 同上 |
| 优先级标签（P0/P1/P2） | 同上 |

看板列（自定义「状态」字段）：`待开始 / 进行中 / 阻塞 / 已完成`，与 Issue 状态标签一一对应。

看板规则：
- Issue 命名：`[模块][优先级][状态] 任务标题`
- 每个任务必须有模块 + 状态标签，关键任务挂「8周实施计划」里程碑
- 完成代码改动并提交后，同步更新 Issue 状态/勾选验收项（与文档同步约束一致）
- 遗留问题（BUILD_PLAN 相关 #1/#2）已过时，以 docs/ 下任务卡与看板为准

> **Token 说明（2026-08-03）：** 早期使用 Fine-grained PAT（个人账户）无法访问 user-owned Projects，
> 已切换为**经典 token（含 project 权限）**并成功通过 API 创建 Project v2 看板。
> 注意：token 仅存于本地 `.git/config`（remote 地址），**严禁提交到仓库**，如泄露请在 GitHub
> Settings → Developer settings → Tokens 中吊销重建。

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
- [第2周开发任务卡](docs/第2周开发任务卡.md) — M2 数据同步（已完成）
- [第3-4周开发任务卡](docs/第3-4周开发任务卡.md) — M3 质控引擎 + M4 消息推送
- [BUILD_PLAN.md](BUILD_PLAN.md) — 构建方案
