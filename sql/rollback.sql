-- ===================================================
-- 住院病例质控系统 - 数据库回滚脚本
-- 用途：出错时按依赖逆序删除所有表
-- 注意：生产环境慎用！仅限开发阶段
-- 日期：2026-07-31
-- ===================================================

USE HospitalQC;
GO

PRINT '开始回滚建表操作...';
GO

-- 7. 确认整改记录表（依赖 inpatient_case）
IF OBJECT_ID('dbo.qc_confirm', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.qc_confirm;
    PRINT '  [OK] DROP TABLE qc_confirm';
END
ELSE
    PRINT '  [SKIP] qc_confirm 不存在';
GO

-- 6. 推送记录表
IF OBJECT_ID('dbo.push_log', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.push_log;
    PRINT '  [OK] DROP TABLE push_log';
END
ELSE
    PRINT '  [SKIP] push_log 不存在';
GO

-- 5. 医生-企业微信映射表
IF OBJECT_ID('dbo.doctor_wework', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.doctor_wework;
    PRINT '  [OK] DROP TABLE doctor_wework';
END
ELSE
    PRINT '  [SKIP] doctor_wework 不存在';
GO

-- 4. 质控结果表（依赖 inpatient_case + qc_rule）
IF OBJECT_ID('dbo.qc_result', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.qc_result;
    PRINT '  [OK] DROP TABLE qc_result';
END
ELSE
    PRINT '  [SKIP] qc_result 不存在';
GO

-- 3. 质控规则表
IF OBJECT_ID('dbo.qc_rule', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.qc_rule;
    PRINT '  [OK] DROP TABLE qc_rule';
END
ELSE
    PRINT '  [SKIP] qc_rule 不存在';
GO

-- 2. 住院病例表
IF OBJECT_ID('dbo.inpatient_case', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.inpatient_case;
    PRINT '  [OK] DROP TABLE inpatient_case';
END
ELSE
    PRINT '  [SKIP] inpatient_case 不存在';
GO

-- 1. 科室表
IF OBJECT_ID('dbo.department', 'U') IS NOT NULL
BEGIN
    DROP TABLE dbo.department;
    PRINT '  [OK] DROP TABLE department';
END
ELSE
    PRINT '  [SKIP] department 不存在';
GO

PRINT '回滚操作完成。';
PRINT '提示：如需重新建表，请按顺序执行 001_schema.sql → 002_init_data.sql → 003_seed.sql';
GO
