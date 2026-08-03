# CLAUDE.md — 住院病例质控企业微信推送系统

## 核心约束

### 🔒 敏感信息安全

**严禁向 GitHub 仓库提交以下内容：**

1. **院方真实数据** — 不得提交任何医院的真实病例数据、患者信息、医生信息
2. **企业微信真实凭证** — 不得提交 CorpID、AgentID、AgentSecret、access_token 等
3. **数据库真实连接信息** — 不得提交真实的数据库 IP、端口、库名、用户名、密码
4. **院方网络拓扑** — 不得提交真实的 IP 地址、服务器名、网段信息

### ✅ 允许提交的内容

- 配置文件的**示例模板**（如 `config.yaml` 中的占位值 `localhost`、`10.x.x.x`）
- 连接示例代码（使用占位变量或环境变量引用）
- 数据库 Schema DDL（不含真实数据）
- 接口文档、设计文档（脱敏后）

### 环境变量策略

- **数据库连接信息**（HIS / 业务库）：由**院方技术人员直接在 `config.yaml` 中填写**（院方要求）。GitHub 提交版本使用占位符（`localhost`、空密码），**严禁提交真实 IP/密码**。
- **企业微信凭证 / JWT**：通过环境变量注入：

```
WEWORK_CORP_ID  # 企业微信 CorpID
WEWORK_AGENT_ID # 企业微信 AgentID
WEWORK_AGENT_SECRET # 企业微信 Secret
JWT_SECRET      # JWT 签名密钥
```

### 配置模板策略

`config.yaml` 提交到 GitHub 时敏感字段使用占位值；院方部署时直接在服务器上填写：

```yaml
database:
  server: "localhost"   # 院方部署时填写实际地址
  user: "sa"            # 院方填写登录账号
  password: ""          # 院方填写密码

his_database:
  server: "localhost"   # 与业务库同一台服务器（单机部署）
  name: ""              # 院方填写 HIS 数据仓库库名
  user: ""              # 院方填写只读账号
  password: ""          # 院方填写只读密码
```

> 身份验证：SQL Server 身份验证；`encrypt=false`（不加密，SQL Server 2014 兼容）。

---

### 📋 文档同步约束（2026-08-03 新增，核心约束）

**每次更新或新增项目源码内容后，必须同步更新项目文档并记录当前进度，不允许只改代码不改文档。**

1. 修改 `docs/项目模块分解与交接文档.md`（主控文档）：
   - 对应模块的「状态」「交付物」勾选情况
   - 新问题/报错写入对应模块的「技术瓶颈 / 报错记录」表
   - 在「八、更新日志」追加一行（日期 / 更新人 / 更新内容）
2. 涉及里程碑推进时，同步更新对应「开发任务卡」（如 `docs/第3-4周开发任务卡.md`）
3. 涉及接口变化时，同步更新 `docs/api.md`
4. 文档更新应与代码改动落入同一提交（或紧随其后的提交），保持「代码-文档」同步可追溯

---

## 项目信息

| 项目 | 内容 |
|------|------|
| 项目名 | 住院病例质控企业微信推送系统 |
| GitHub | https://github.com/MIutopia/hospital-qc-wework |
| 分支策略 | main(受保护) ← feature/* PR |
| 提交规范 | Conventional Commits |
| 文档入口 | docs/项目模块分解与交接文档.md |

## 技术栈

- 后端：Go 1.22+ / Gin / go-mssqldb / expr-lang/expr / robfig/cron / zerolog
- 数据库：SQL Server 2014（NVARCHAR(MAX) 存 JSON）
- 部署：Windows 原生（NSSM + Nginx for Windows）
- 前端：Vue 3 + Vant 4 + Vite（M5 阶段）

## 开发命令

```bash
# 后端启动（开发）
cd backend
go run cmd/server/main.go --config config.yaml

# 构建 Windows 可执行文件
cd backend && go build -o hospital-qc.exe cmd/server/main.go

# 运行测试
cd backend && go test ./...
```
