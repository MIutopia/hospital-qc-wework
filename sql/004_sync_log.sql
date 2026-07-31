-- ===================================================
-- 住院病例质控系统 - 同步日志表（M2）
-- 数据库：SQL Server 2014
-- 说明：记录 HIS 增量同步 / CSV 导入的执行历史，供管理后台查询与故障诊断
-- ===================================================

USE HospitalQC;
GO

IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[sync_log]') AND type in (N'U'))
BEGIN
    CREATE TABLE sync_log (
        id            BIGINT IDENTITY(1,1) PRIMARY KEY,
        sync_type     VARCHAR(16)   NOT NULL,       -- HIS / CSV
        status        VARCHAR(16)   NOT NULL,       -- RUNNING / SUCCESS / FAILED
        total_synced  INT           DEFAULT 0,
        new_cases     INT           DEFAULT 0,
        updated       INT           DEFAULT 0,
        error_msg     NVARCHAR(MAX) NULL,           -- 失败原因（如有）
        started_at    DATETIME2     DEFAULT GETDATE(),
        finished_at   DATETIME2     NULL,
        elapsed_ms    BIGINT        NULL
    );

    CREATE INDEX idx_sync_log_time ON sync_log (started_at DESC);
    CREATE INDEX idx_sync_log_type ON sync_log (sync_type, status);
END
GO

PRINT '同步日志表创建完成。';
GO
