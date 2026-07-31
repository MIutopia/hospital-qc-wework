-- ===================================================
-- 住院病例质控企业微信推送系统 - 数据库建表脚本
-- 数据库：SQL Server 2014
-- 日期：2026-07-31
-- 说明：
--   1. SQL Server 2014 无原生 JSON 类型
--      JSON 字段使用 NVARCHAR(MAX) 存储，应用层解析
--   2. updated_at 由应用层在 UPDATE 时显式赋值
--   3. 不使用 ON UPDATE CURRENT_TIMESTAMP
-- ===================================================

-- 创建数据库（如已有则跳过）
IF NOT EXISTS (SELECT name FROM sys.databases WHERE name = 'HospitalQC')
BEGIN
    CREATE DATABASE HospitalQC;
END
GO

USE HospitalQC;
GO

-- ===================================================
-- 1. 科室表
-- ===================================================
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[department]') AND type in (N'U'))
BEGIN
    CREATE TABLE department (
        id           BIGINT IDENTITY(1,1) PRIMARY KEY,
        dept_name    NVARCHAR(128)  NOT NULL,
        dept_code    VARCHAR(64)    NULL,
        parent_id    BIGINT         NULL,
        created_at   DATETIME2      DEFAULT GETDATE(),
        updated_at   DATETIME2      DEFAULT GETDATE()
    );

    CREATE INDEX idx_dept_parent ON department (parent_id);
END
GO

-- ===================================================
-- 2. 住院病例表
-- ===================================================
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[inpatient_case]') AND type in (N'U'))
BEGIN
    CREATE TABLE inpatient_case (
        id              BIGINT IDENTITY(1,1) PRIMARY KEY,
        case_no         VARCHAR(64)    NOT NULL,
        patient_name    NVARCHAR(128)  NOT NULL,
        patient_gender  TINYINT        NULL,
        patient_age     INT            NULL,
        admit_time      DATETIME2      NOT NULL,
        discharge_time  DATETIME2      NULL,
        dept_id         BIGINT         NOT NULL,
        dept_name       NVARCHAR(128)  NULL,
        doctor_id       BIGINT         NULL,
        doctor_name     NVARCHAR(64)   NULL,
        diagnosis       NVARCHAR(MAX)  NULL,
        case_status     VARCHAR(32)    DEFAULT 'ACTIVE',
        raw_data        NVARCHAR(MAX)  NULL,           -- JSON 字符串，应用层解析
        sync_time       DATETIME2      DEFAULT GETDATE(),
        qc_status       VARCHAR(32)    DEFAULT 'PENDING',  -- PENDING/PASSED/ISSUED
        qc_time         DATETIME2      NULL,
        created_at      DATETIME2      DEFAULT GETDATE(),
        updated_at      DATETIME2      DEFAULT GETDATE()
    );

    -- 约束与索引
    ALTER TABLE inpatient_case ADD CONSTRAINT uk_case_no UNIQUE (case_no);
    CREATE INDEX idx_qc_status ON inpatient_case (qc_status);
    CREATE INDEX idx_doctor_id ON inpatient_case (doctor_id);
    CREATE INDEX idx_dept_id ON inpatient_case (dept_id);
    CREATE INDEX idx_admit_time ON inpatient_case (admit_time);
END
GO

-- ===================================================
-- 3. 质控规则表
-- ===================================================
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[qc_rule]') AND type in (N'U'))
BEGIN
    CREATE TABLE qc_rule (
        id               BIGINT IDENTITY(1,1) PRIMARY KEY,
        rule_code        VARCHAR(64)    NOT NULL,
        rule_name        NVARCHAR(256)  NOT NULL,
        rule_category    VARCHAR(64)    NOT NULL,       -- TIMELINESS/COMPLETENESS/LOGIC/CONSISTENCY
        rule_level       VARCHAR(32)    NOT NULL,       -- A(严重)/B(一般)/C(提示)
        rule_expression  NVARCHAR(MAX)  NOT NULL,       -- JSON DSL，应用层解析
        rule_desc        NVARCHAR(MAX)  NULL,
        is_enabled       TINYINT        DEFAULT 1,
        priority         INT            DEFAULT 0,
        created_at       DATETIME2      DEFAULT GETDATE(),
        updated_at       DATETIME2      DEFAULT GETDATE()
    );

    ALTER TABLE qc_rule ADD CONSTRAINT uk_rule_code UNIQUE (rule_code);
    CREATE INDEX idx_enabled_priority ON qc_rule (is_enabled, priority);
END
GO

-- ===================================================
-- 4. 质控结果表
-- ===================================================
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[qc_result]') AND type in (N'U'))
BEGIN
    CREATE TABLE qc_result (
        id               BIGINT IDENTITY(1,1) PRIMARY KEY,
        case_id          BIGINT         NOT NULL,
        rule_id          BIGINT         NOT NULL,
        is_defect        TINYINT        NOT NULL,       -- 0:否 1:是
        defect_detail    NVARCHAR(MAX)  NULL,
        defect_location  NVARCHAR(256)  NULL,
        suggestion       NVARCHAR(MAX)  NULL,
        qc_batch_id      VARCHAR(64)    NULL,
        created_at       DATETIME2      DEFAULT GETDATE(),
        CONSTRAINT fk_qc_result_case FOREIGN KEY (case_id) REFERENCES inpatient_case(id),
        CONSTRAINT fk_qc_result_rule FOREIGN KEY (rule_id) REFERENCES qc_rule(id)
    );

    CREATE INDEX idx_qc_case_id ON qc_result (case_id);
    CREATE INDEX idx_qc_batch_id ON qc_result (qc_batch_id);
    CREATE INDEX idx_qc_defect ON qc_result (case_id, is_defect);
END
GO

-- ===================================================
-- 5. 医生-企业微信映射表
-- ===================================================
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[doctor_wework]') AND type in (N'U'))
BEGIN
    CREATE TABLE doctor_wework (
        id              BIGINT IDENTITY(1,1) PRIMARY KEY,
        doctor_id       BIGINT         NOT NULL,
        doctor_name     NVARCHAR(64)   NOT NULL,
        dept_id         BIGINT         NULL,
        wework_userid   VARCHAR(128)   NOT NULL,
        phone           VARCHAR(32)    NULL,
        is_active       TINYINT        DEFAULT 1,
        created_at      DATETIME2      DEFAULT GETDATE(),
        updated_at      DATETIME2      DEFAULT GETDATE()
    );

    ALTER TABLE doctor_wework ADD CONSTRAINT uk_doctor_id UNIQUE (doctor_id);
    ALTER TABLE doctor_wework ADD CONSTRAINT uk_wework_userid UNIQUE (wework_userid);
    CREATE INDEX idx_wework_dept ON doctor_wework (dept_id);
END
GO

-- ===================================================
-- 6. 推送记录表
-- ===================================================
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[push_log]') AND type in (N'U'))
BEGIN
    CREATE TABLE push_log (
        id               BIGINT IDENTITY(1,1) PRIMARY KEY,
        case_id          BIGINT         NOT NULL,
        qc_result_ids    NVARCHAR(MAX)  NOT NULL,       -- JSON 数组字符串
        receiver_userid  VARCHAR(128)   NOT NULL,
        push_type        VARCHAR(32)    NOT NULL,       -- CARD/MARKDOWN/TEXT
        push_content     NVARCHAR(MAX)  NULL,           -- JSON 字符串
        push_status      VARCHAR(32)    DEFAULT 'PENDING', -- PENDING/SUCCESS/FAILED/DEFERRED
        push_response    NVARCHAR(MAX)  NULL,
        retry_count      INT            DEFAULT 0,
        pushed_at        DATETIME2      NULL,
        created_at       DATETIME2      DEFAULT GETDATE()
    );

    CREATE INDEX idx_push_case ON push_log (case_id);
    CREATE INDEX idx_push_receiver ON push_log (receiver_userid);
    CREATE INDEX idx_push_status ON push_log (push_status, retry_count);
END
GO

-- ===================================================
-- 7. 确认整改记录表（M5 阶段使用）
-- ===================================================
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[qc_confirm]') AND type in (N'U'))
BEGIN
    CREATE TABLE qc_confirm (
        id              BIGINT IDENTITY(1,1) PRIMARY KEY,
        case_id         BIGINT         NOT NULL,
        doctor_id       BIGINT         NOT NULL,
        defect_ids      NVARCHAR(MAX)  NULL,           -- JSON 数组，确认的缺陷ID列表
        confirm_status  VARCHAR(32)    DEFAULT 'PENDING',  -- PENDING/CONFIRMED/REJECTED
        confirm_note    NVARCHAR(MAX)  NULL,
        confirmed_at    DATETIME2      NULL,
        created_at      DATETIME2      DEFAULT GETDATE(),
        updated_at      DATETIME2      DEFAULT GETDATE(),
        CONSTRAINT fk_confirm_case FOREIGN KEY (case_id) REFERENCES inpatient_case(id)
    );

    CREATE INDEX idx_confirm_case ON qc_confirm (case_id, doctor_id);
END
GO

PRINT '数据库建表脚本执行完成。';
GO
