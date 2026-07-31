# 住院病例质控企业微信推送系统

某医院住院部每日产生大量住院病例。质控科通过本系统对病例进行规则化质量审核，发现问题后精准推送给责任医生，医生在企业微信中点击消息即可查看详细质控报告并确认整改。

## 核心流程

```
HIS 数据库 → 数据同步 → 质控规则引擎 → 企业微信消息推送 → 医生 H5 页面确认
```

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
├── sql/              # 数据库脚本
├── deploy/           # 部署配置
└── docs/             # 项目文档
```

## 快速开始

```bash
# 后端
cd backend
go mod tidy
go run cmd/server/main.go --config config.yaml

# 前端
cd frontend
npm install
npm run dev
```

## 文档

- [项目模块分解与交接文档](docs/项目模块分解与交接文档.md) — 项目管理的总控文档
- [API 接口文档](docs/api.md) — 全部接口定义

## 许可

内部项目，专为医院部署使用。
