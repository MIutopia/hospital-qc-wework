-- ===================================================
-- 住院病例质控系统 - 测试数据脚本
-- 说明：生成脱敏测试数据用于开发和联调
-- 注意：仅用于开发和测试环境，禁止用于生产
-- ===================================================

USE HospitalQC;
GO

-- ===================================================
-- 测试医生-企业微信映射
-- ===================================================
IF NOT EXISTS (SELECT 1 FROM doctor_wework)
BEGIN
    INSERT INTO doctor_wework (doctor_id, doctor_name, dept_id, wework_userid, phone) VALUES
        (1001, N'李明',  1, 'zhang_san',  '13800138001'),
        (1002, N'王芳',  1, 'li_si',      '13800138002'),
        (2001, N'赵刚',  2, 'wang_wu',    '13800138003'),
        (2002, N'刘洋',  2, 'zhao_liu',   '13800138004'),
        (3001, N'陈静',  3, 'chen_qi',    '13800138005');
END
GO

-- ===================================================
-- 测试病例数据（脱敏）
-- ===================================================
IF NOT EXISTS (SELECT 1 FROM inpatient_case)
BEGIN
    -- 病例 1：有缺陷（主诉为空）
    INSERT INTO inpatient_case (case_no, patient_name, patient_gender, patient_age, admit_time, discharge_time, dept_id, dept_name, doctor_id, doctor_name, diagnosis, qc_status)
    VALUES (
        'ZY202607001', N'张**', 1, 45, '2026-07-28 10:30:00', '2026-07-30 14:00:00',
        1, N'心内科', 1001, N'李明',
        N'冠状动脉粥样硬化性心脏病',
        'ISSUED'
    );

    -- 病例 2：无缺陷
    INSERT INTO inpatient_case (case_no, patient_name, patient_gender, patient_age, admit_time, discharge_time, dept_id, dept_name, doctor_id, doctor_name, diagnosis, qc_status)
    VALUES (
        'ZY202607002', N'李**', 0, 32, '2026-07-27 08:00:00', '2026-07-29 10:00:00',
        1, N'心内科', 1001, N'李明',
        N'高血压病2级',
        'PASSED'
    );

    -- 病例 3：有缺陷（出院时间早于入院时间）
    INSERT INTO inpatient_case (case_no, patient_name, patient_gender, patient_age, admit_time, discharge_time, dept_id, dept_name, doctor_id, doctor_name, diagnosis, qc_status)
    VALUES (
        'ZY202607003', N'王**', 1, 58, '2026-07-29 14:00:00', '2026-07-28 10:00:00',
        2, N'呼吸内科', 2001, N'赵刚',
        N'慢性阻塞性肺疾病急性加重',
        'ISSUED'
    );

    -- 病例 4：待质控
    INSERT INTO inpatient_case (case_no, patient_name, patient_gender, patient_age, admit_time, discharge_time, dept_id, dept_name, doctor_id, doctor_name, diagnosis, qc_status)
    VALUES (
        'ZY202607004', N'赵**', 0, 28, '2026-07-30 09:00:00', NULL,
        3, N'消化内科', 3001, N'陈静',
        N'急性阑尾炎',
        'PENDING'
    );

    -- 病例 5：待质控
    INSERT INTO inpatient_case (case_no, patient_name, patient_gender, patient_age, admit_time, discharge_time, dept_id, dept_name, doctor_id, doctor_name, diagnosis, qc_status)
    VALUES (
        'ZY202607005', N'刘**', 1, 65, '2026-07-31 11:00:00', NULL,
        2, N'呼吸内科', 2001, N'赵刚',
        N'社区获得性肺炎',
        'PENDING'
    );
END
GO

PRINT '测试数据脚本执行完成。';
GO
