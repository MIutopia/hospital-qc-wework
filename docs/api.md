# API 接口文档 — 住院病例质控企业微信推送系统

---

## 基础信息

- **Base URL：** `https://api.xxx.cn/api/v1`
- **统一响应格式：** `{"code": 0, "message": "ok", "data": {...}}`
- **错误码规范：**
  - `0` — 成功
  - `400xx` — 参数错误
  - `401xx` — 鉴权失败
  - `403xx` — 权限不足
  - `404xx` — 资源不存在
  - `500xx` — 服务端错误
- **鉴权方式：** `Authorization: Bearer <JWT Token>`

---

## 一、公开接口

### 1.1 健康检查

```
GET /api/v1/health
```

**响应示例：**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "status": "ok",
    "version": "1.0.0"
  }
}
```

---

## 二、H5 接口（需 JWT 鉴权）

### 2.1 获取质控报告详情

```
GET /api/v1/report/detail?caseId=12345
Authorization: Bearer <token>
```

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| caseId | int64 | 是 | 病例 ID |

**响应示例：**
```json
{
  "code": 0,
  "data": {
    "caseId": 12345,
    "caseNo": "ZY202607001",
    "patientName": "张**",
    "patientGender": 1,
    "patientAge": 45,
    "deptName": "心内科",
    "doctorName": "李明",
    "admitTime": "2026-07-28T10:30:00",
    "diagnosis": "冠状动脉粥样硬化性心脏病",
    "qcStatus": "ISSUED",
    "defectSummary": {
      "total": 3,
      "levelA": 2,
      "levelB": 1,
      "levelC": 0
    },
    "defects": [
      {
        "id": 101,
        "ruleName": "入院记录缺失主诉",
        "ruleLevel": "A",
        "defectDetail": "入院记录中主诉字段为空",
        "defectLocation": "入院记录 > 主诉",
        "suggestion": "请补充患者主诉内容"
      }
    ],
    "isConfirmed": false
  }
}
```

**错误响应：**

| HTTP 状态码 | code | message | 说明 |
|------------|------|---------|------|
| 404 | 40401 | 病例不存在 | caseId 对应的病例未找到 |
| 401 | 40103 | Token 无效或已过期 | JWT 过期或签名错误 |

---

### 2.2 确认整改

```
POST /api/v1/report/confirm
Authorization: Bearer <token>
Content-Type: application/json

{
  "caseId": 12345
}
```

**响应示例：**
```json
{
  "code": 0,
  "data": {
    "caseId": 12345,
    "confirmed": true
  }
}
```

---

### 2.3 我的待整改列表

```
GET /api/v1/doctor/tasks?page=1&pageSize=20&status=ISSUED
Authorization: Bearer <token>
```

**参数：**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| page | int | 否 | 1 | 页码 |
| pageSize | int | 否 | 20 | 每页条数（最大100） |
| status | string | 否 | ISSUED | 质控状态（PENDING/PASSED/ISSUED） |

**响应示例：**
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 12345,
        "caseNo": "ZY202607001",
        "patientName": "张**",
        "deptName": "心内科",
        "doctorName": "李明",
        "admitTime": "2026-07-28T10:30:00",
        "qcStatus": "ISSUED",
        "QCTime": "2026-07-31T06:15:00"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20
  }
}
```

---

### 2.4 科室统计（选做）

```
GET /api/v1/dept/stats
Authorization: Bearer <token>
```

**参数：**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| date | string | 否 | 今日 | 统计日期 YYYY-MM-DD |

**响应示例：**
```json
{
  "code": 0,
  "data": {
    "deptName": "心内科",
    "totalCases": 50,
    "defectCases": 12,
    "defectRate": "24.00%",
    "levelADefects": 5,
    "levelBDefects": 7,
    "confirmedRate": "75.00%"
  }
}
```

---

## 三、管理接口

### 3.1 规则 CRUD

#### 规则列表

```
GET /api/v1/admin/rules?page=1&pageSize=20
```

#### 创建规则

```
POST /api/v1/admin/rules
Content-Type: application/json

{
  "ruleCode": "TIMELINESS_001",
  "ruleName": "入院记录24h内完成",
  "ruleCategory": "TIMELINESS",
  "ruleLevel": "A",
  "ruleExpression": "{...}",
  "ruleDesc": "...",
  "isEnabled": 1,
  "priority": 1
}
```

#### 更新规则

```
PUT /api/v1/admin/rules/:id
Content-Type: application/json

{
  "ruleName": "入院记录24h内完成（更新）",
  "isEnabled": 0,
  ...
}
```

#### 删除规则

```
DELETE /api/v1/admin/rules/:id
```

---

### 3.2 数据同步

#### 手动触发同步（HIS 增量拉取）

```
POST /api/v1/admin/sync
```

从 HIS 数据仓库 `med_record` 库增量同步住院病例到业务库 `inpatient_case`。
增量断点 = 业务库最大 `sync_time`；首次同步取最近 30 天。

**响应示例：**
```json
{
  "code": 0,
  "data": {
    "totalSynced": 50,
    "newCases": 5,
    "updated": 3,
    "elapsed": "3.2s"
  }
}
```

**错误响应：**

| HTTP 状态码 | code | message | 说明 |
|------------|------|---------|------|
| 500 | 50001 | HIS 数据库未配置连接（请设置 HIS_DB_USER / HIS_DB_PASS 环境变量） | HIS 连接未配置 |

---

#### CSV 手工导入（阶段一兜底）

```
POST /api/v1/admin/sync/csv
Content-Type: application/x-www-form-urlencoded

path=D:\data\cases.csv
```

**CSV 支持的中文表头：**

| 列名 | 必填 | 说明 |
|------|------|------|
| 住院号 | 是 | 病例唯一标识 |
| 姓名 | 是 | 患者姓名 |
| 性别 | 否 | 男/女 |
| 年龄 | 否 | 整数 |
| 入院时间 | 否 | 支持多种日期格式 |
| 出院时间 | 否 | 同上 |
| 入院科室 | 否 | 匹配本地科室表 |
| 住院医师 | 否 | 匹配医生映射表 |
| 西医初步诊断 | 否 | 诊断信息 |
| 主诉 | 否 | 存入 raw_data |

**响应示例：** 同手动触发同步。

---

### 3.3 手动触发质控

```
POST /api/v1/admin/qc/run
```

**响应示例：**
```json
{
  "code": 0,
  "data": {
    "batchId": "QC_20260801_001",
    "totalCases": 187,
    "defectCases": 12,
    "totalDefects": 23,
    "passedCases": 175,
    "elapsed": "12.3s"
  }
}
```

---

### 3.4 推送日志

```
GET /api/v1/admin/push/logs?page=1&pageSize=20&status=FAILED
```

**参数：**

| 参数 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| page | int | 否 | 1 | 页码 |
| pageSize | int | 否 | 20 | 每页条数 |
| status | string | 否 | 全部 | PENDING/SUCCESS/FAILED/DEFERRED |

---

### 3.5 医生映射管理

#### 医生列表

```
GET /api/v1/admin/doctors?page=1&pageSize=20
```

#### 创建/更新医生映射

```
POST /api/v1/admin/doctors
Content-Type: application/json

{
  "doctorId": 1001,
  "doctorName": "李明",
  "deptId": 1,
  "weworkUserid": "zhang_san",
  "phone": "13800138001",
  "isActive": 1
}
```

---

## 四、错误码一览

| code | 说明 |
|------|------|
| 0 | 成功 |
| 40001 | 请求参数错误 |
| 40101 | 缺少 Authorization 头 |
| 40102 | Authorization 格式错误 |
| 40103 | Token 无效或已过期 |
| 40301 | 权限不足 |
| 40401 | 资源不存在 |
| 50001 | 服务器内部错误 |
| 50002 | 数据库操作失败 |
| 50003 | 外部 API 调用失败 |
