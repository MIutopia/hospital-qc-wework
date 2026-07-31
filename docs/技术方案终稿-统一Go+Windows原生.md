<!--
  文档名称：住院病例质控企业微信推送系统 - 技术方案终稿
  版本：V1.1（终稿）
  日期：2026-07-31
  前置阅读：PM决策备忘录-技术方案对齐.md
  状态：✅ 已对齐，可进入实施
-->

# 住院病例质控企业微信推送系统 — 技术方案终稿

> 本文档为 PM 决策对齐后的技术方案终稿。所有与决策不一致的历史残留均已修正。

---

## 一、技术栈（最终确认）

| 层级 | 选型 | 版本 | 决策来源 |
|------|------|------|---------|
| **后端语言** | Go | 1.22+ | PM 决策 1 |
| **Web 框架** | Gin | v1.10+ | 连带决策 |
| **数据库** | SQL Server 2014 | 院方现有 | 方案一致 |
| **数据库驱动** | go-mssqldb + sqlx | - | 连带决策 |
| **缓存** | Memurai / 进程内 MemoryCache | - | PM 决策 2 |
| **规则引擎** | expr-lang/expr | v1 | PM 决策 4 |
| **任务调度** | robfig/cron | v3 | PM 决策 3 |
| **JWT** | golang-jwt/jwt | v5 | 连带决策 |
| **日志** | zerolog | v1 | 连带决策 |
| **前端** | Vue 3 + Vant 4 + Vite | 最新 | 方案一致 |
| **部署** | Windows Service (NSSM) + Nginx for Windows | - | PM 决策 2 |

> **以上技术栈已全部对齐，不再存在 Go/Java/C# 多选一的情况。**

---

## 二、系统架构

### 2.1 总体架构

```mermaid
flowchart LR
    HIS[("HIS 数据库<br/>SQL Server 2014")] -->|JDBC 定时同步| Backend

    subgraph Backend["应用服务器 Win Server 2019"]
        direction TB
        Sync["数据同步<br/>sync_service"] --> DB[("业务数据库<br/>HospitalQC")]
        Scheduler["调度器<br/>robfig/cron"] --> Sync
        Scheduler --> Engine
        Scheduler --> Pusher
        Engine["质控引擎<br/>qc_engine + expr"] --> DB
        Mem["Memurai<br/>或 MemoryCache"] -.->|缓存 access_token| Pusher
        DB --> API["REST API<br/>Gin"]
    end

    Backend -->|HTTPS| WE["企业微信 API"]
    API -->|JSON| H5["Vue 3 + Vant H5"]

    WE -->|模板卡片| DOC["医生企业微信"]
    DOC -->|点击跳转| H5
```

### 2.2 网络拓扑（单机部署）

> **院方要求（2026-07-31）：** 项目部署到和数据库同一个服务器上。
> 应用服务器 = 数据库服务器 = 服务器实例 `WIN-OKT2FAKMULV\SQLEXPRESS`。

```mermaid
graph TD
    subgraph 内网单机["WIN-OKT2FAKMULV 单机（应用 + 数据库 + 中间件）"]
        HIS[("HIS 数据仓库<br/>med_record / records / caiwu")]
        DB[("HospitalQC<br/>业务数据库<br/>localhost:1433")]
        APP["后端服务<br/>hospital-qc.exe<br/>localhost:8080"]
        NGX["Nginx for Windows<br/>本地反向代理"]
        MEM["Memurai / MemoryCache<br/>localhost:6379"]
    end

    subgraph 公网
        WX["qyapi.weixin.qq.com"]
    end

    APP -->|本机只读| HIS
    APP -->|本机读写| DB
    APP -->|缓存| MEM
    NGX -->|本地反代| APP
    NGX -->|HTTPS 出站| WX
```

---

## 三、项目结构（最终版）

```
hospital-qc-wework/
├── backend/                          # Go 后端
│   ├── cmd/server/main.go            # 入口
│   ├── internal/
│   │   ├── config/                   # 配置管理
│   │   ├── model/                    # 数据模型
│   │   │   ├── inpatient_case.go
│   │   │   ├── qc_rule.go
│   │   │   ├── qc_result.go
│   │   │   ├── doctor_wework.go
│   │   │   ├── push_log.go
│   │   │   └── department.go
│   │   ├── dao/                      # 数据库访问层
│   │   │   ├── db.go
│   │   │   ├── case_dao.go
│   │   │   ├── rule_dao.go
│   │   │   ├── result_dao.go
│   │   │   ├── doctor_dao.go
│   │   │   └── push_log_dao.go
│   │   ├── middleware/               # HTTP 中间件
│   │   │   ├── auth.go
│   │   │   ├── logger.go
│   │   │   └── recovery.go
│   │   ├── handler/                  # HTTP 处理器
│   │   │   ├── routes.go
│   │   │   ├── report_handler.go
│   │   │   ├── doctor_handler.go
│   │   │   └── admin_handler.go
│   │   └── service/                  # 业务逻辑
│   │       ├── sync/                 # 数据同步
│   │       ├── qc/
│   │       │   ├── dsl.go            # DSL 解析 + expr 编译
│   │       │   └── engine.go         # 质控执行器
│   │       ├── push/
│   │       │   ├── token.go          # access_token 管理
│   │       │   └── pusher.go         # 消息发送 + 限频
│   │       └── auth/
│   │           └── jwt.go            # JWT 鉴权
│   ├── pkg/                          # 公共工具包
│   │   ├── response/response.go
│   │   └── tokenbucket/bucket.go
│   ├── config.yaml
│   └── go.mod
│
├── frontend/                         # Vue 3 前端
│   └── src/
│       ├── views/                    # 页面（ReportDetail、MyTasks、DeptStats、AuthError）
│       ├── components/               # 组件（PatientCard、DefectList、RawContent、ConfirmButton）
│       ├── api/                      # API 封装
│       ├── router/                   # 路由
│       └── utils/                    # 工具函数
│
├── sql/                              # 数据库脚本
│   ├── 001_schema.sql
│   ├── 002_init_data.sql
│   └── 003_seed.sql
│
├── deploy/                           # 部署配置
│   ├── nginx.conf
│   └── build.bat
│
├── docs/                             # 文档
│   ├── 项目模块分解与交接文档.md
│   ├── 第1周开发任务卡.md
│   ├── api.md
│   └── 技术方案终稿.md              # ← 当前文档
│
├── .gitignore
└── README.md
```

---

## 四、核心业务流

### 4.1 全链路时间线

```mermaid
flowchart LR
    A["06:00 数据同步<br/>sync_service"] --> B["06:10 质控执行<br/>qc_engine"]
    B --> C["06:25 结果聚合<br/>按医生分组"]
    C --> D["06:30 批量推送<br/>push_service"]
    D --> E["医生收到消息<br/>→ 点击卡片"]
    E --> F["H5 报告详情<br/>→ 确认整改"]
```

### 4.2 质控引擎内部流程

```mermaid
flowchart TD
    A["定时触发/cron<br/>POST /admin/qc/run"] --> B["生成批次号"]
    B --> C["查询待质控病例"]
    C --> D["加载全部启用规则<br/>→ 内存缓存"]
    D --> E["goroutine 池(10)<br/>并发执行"]
    E --> F["解析 DSL<br/>expr.Compile"]
    F --> G["expr.Run 求值"]
    G --> H{"is_defect?"}
    H -->|是| I["INSERT qc_result"]
    H -->|否| J["跳过"]
    I --> K["UPDATE case<br/>qc_status=ISSUED"]
    J --> K
    K --> L{"有缺陷?"}
    L -->|是| M["触发推送"]
    L -->|否| N["qc_status=PASSED<br/>结束"]
```

### 4.3 消息推送流程

```mermaid
flowchart LR
    subgraph 推送服务
        A["聚合缺陷<br/>按医生+病例"] --> B["令牌桶限频<br/>10次/秒"]
        B --> C["发送模板卡片<br/>POST message/send"]
        C --> D{"成功?"}
        D -->|是| E["push_log<br/>status=SUCCESS"]
        D -->|否| F["指数退避重试<br/>1m→5m→15m→30m"]
        F --> D
        F -->|4次均失败| G["push_log<br/>status=FAILED<br/>人工介入"]
    end

    subgraph 免打扰
        H{"22:00~08:00?"}
        H -->|是| I["status=DEFERRED<br/>08:00后继续推送"]
        H -->|否| A
    end
```

---

## 五、数据库设计

### 5.1 ER 关系

```mermaid
erDiagram
    DEPT ||--o{ DOCTOR_WEWORK : "1:N"
    DEPT ||--o{ INPATIENT_CASE : "1:N"
    DOCTOR_WEWORK ||--o{ INPATIENT_CASE : "1:N"
    INPATIENT_CASE ||--o{ QC_RESULT : "1:N"
    QC_RULE ||--o{ QC_RESULT : "1:N"
    QC_RESULT ||--o{ PUSH_LOG : "1:N"
    INPATIENT_CASE ||--o{ QC_CONFIRM : "1:N"
```

### 5.2 核心表清单

| 表名 | 说明 | 关键字段 |
|------|------|---------|
| `department` | 科室表 | id, dept_name, dept_code |
| `inpatient_case` | 住院病例表 | case_no, patient_name, dept_id, doctor_id, qc_status |
| `qc_rule` | 质控规则表 | rule_code, rule_level, rule_expression (JSON DSL) |
| `qc_result` | 质控结果表 | case_id, rule_id, is_defect, defect_detail |
| `doctor_wework` | 医生企业微信映射 | doctor_id, wework_userid |
| `push_log` | 推送记录表 | case_id, receiver_userid, push_status |
| `qc_confirm` | 确认整改记录 | case_id, doctor_id, confirm_status |

> 注意：SQL Server 2014 无原生 JSON 类型，所有 JSON 字段使用 `NVARCHAR(MAX)` 存储，应用层解析。

---

## 六、安全设计

| 关注点 | 方案 |
|--------|------|
| 传输加密 | 全站 HTTPS，Nginx 反向代理终结 TLS |
| 患者隐私 | 姓名脱敏（首字+`**`），日志不落明文 |
| 鉴权 | JWT + HMAC-SHA256，24h 过期 |
| 密钥管理 | DB_PASS / WEWORK_SECRET / JWT_SECRET 仅从环境变量读取 |
| SQL 注入 | sqlx 参数化查询，不使用字符串拼接 |
| 日志脱敏 | zerolog 自定义 hook，过滤敏感字段 |
| 限频 | 令牌桶 10次/秒，保护企业微信 API |

---

## 七、实施计划

```mermaid
gantt
    title 8周实施计划（终稿）
    dateFormat  YYYY-MM-DD
    axisFormat  %m/%d

    section M1 基础设施
    第1周                    :m1, 2026-08-04, 5d

    section M2 数据同步
    第2-3周                  :m2, 2026-08-11, 10d

    section M3 质控引擎
    第3-4周                  :m3, 2026-08-18, 10d

    section M4 消息推送
    第4-5周                  :m4, 2026-08-25, 10d

    section M5 H5 前端
    第4-6周                  :m5, 2026-09-01, 12d

    section M6 联调测试
    第7周                    :m6, 2026-09-15, 5d

    section M7 上线
    第8周                    :m7, 2026-09-22, 5d
```

---

## 八、与产品方案不一致项的修正记录

| 产品方案 V1.0 原内容 | 修正后 | 修正依据 |
|---------------------|--------|---------|
| 后端 Go/Java/C# 混写 | **统一为 Go** | PM 决策 1 |
| 部署 IIS / Docker 混写 | **Windows 原生 (NSSM + Nginx)** | PM 决策 2 |
| 任务调度 Windows 任务计划程序 | **应用内调度 robfig/cron** | PM 决策 3 |
| 规则 DSL C# 解析 | **Go expr-lang/expr 解析** | PM 决策 4 |
| HIS 同步阻塞 M1-M2 | **三阶段解耦，M1 先用 CSV** | PM 决策 5 |
