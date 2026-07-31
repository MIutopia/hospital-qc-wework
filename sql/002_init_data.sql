-- ===================================================
-- 住院病例质控系统 - 初始数据脚本
-- 数据库：SQL Server 2014
-- 说明：初始化科室数据和示例质控规则
-- ===================================================

USE HospitalQC;
GO

-- ===================================================
-- 科室基础数据
-- ===================================================
IF NOT EXISTS (SELECT 1 FROM department)
BEGIN
    INSERT INTO department (dept_name, dept_code) VALUES
        (N'心内科', 'CARDIO'),
        (N'呼吸内科', 'RESP'),
        (N'消化内科', 'GI'),
        (N'神经内科', 'NEURO'),
        (N'普通外科', 'GEN_SURG'),
        (N'骨科', 'ORTHO'),
        (N'妇产科', 'OBGYN'),
        (N'儿科', 'PED');
END
GO

-- ===================================================
-- 示例质控规则（首批 10 条核心规则）
-- ===================================================
IF NOT EXISTS (SELECT 1 FROM qc_rule)
BEGIN
    -- 1. 时效性 - 入院记录24h内完成
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'TIMELINESS_001',
        N'入院记录24h内完成',
        'TIMELINESS',
        'A',
        '{"ruleCode":"TIMELINESS_001","ruleName":"入院记录24h内完成","category":"TIMELINESS","targetField":"raw_data.admission_record.create_time","operator":"HOURS_SINCE","referenceField":"admit_time","threshold":24,"condition":"GREATER_THAN","defectTemplate":"入院记录未在规定24h内完成，实际耗时 {actual} 小时","suggestion":"请在患者入院24小时内完成入院记录书写"}',
        N'检查入院记录是否在入院后24小时内完成',
        1
    );

    -- 2. 完整性 - 主诉不可为空
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'COMPLETENESS_001',
        N'主诉不可为空',
        'COMPLETENESS',
        'A',
        '{"ruleCode":"COMPLETENESS_001","ruleName":"主诉不可为空","category":"COMPLETENESS","targetField":"raw_data.admission_record.complaint","operator":"IS_NULL","threshold":null,"condition":null,"defectTemplate":"入院记录中主诉字段为空","suggestion":"请补充患者主诉内容"}',
        N'检查入院记录中主诉是否存在',
        2
    );

    -- 3. 完整性 - 现病史不可为空
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'COMPLETENESS_002',
        N'现病史不可为空',
        'COMPLETENESS',
        'A',
        '{"ruleCode":"COMPLETENESS_002","ruleName":"现病史不可为空","category":"COMPLETENESS","targetField":"raw_data.admission_record.hpi","operator":"IS_NULL","threshold":null,"condition":null,"defectTemplate":"入院记录中现病史字段为空","suggestion":"请补充患者现病史内容"}',
        N'检查入院记录中现病史是否存在',
        3
    );

    -- 4. 完整性 - 诊断不可为空
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'COMPLETENESS_003',
        N'出院诊断不可为空',
        'COMPLETENESS',
        'A',
        '{"ruleCode":"COMPLETENESS_003","ruleName":"出院诊断不可为空","category":"COMPLETENESS","targetField":"diagnosis","operator":"IS_NULL","threshold":null,"condition":null,"defectTemplate":"出院诊断字段为空","suggestion":"请填写出院诊断"}',
        N'检查出院诊断是否存在',
        4
    );

    -- 5. 逻辑性 - 出院时间不可早于入院时间
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'LOGIC_001',
        N'出院时间不可早于入院时间',
        'LOGIC',
        'B',
        '{"ruleCode":"LOGIC_001","ruleName":"出院时间不可早于入院时间","category":"LOGIC","targetField":"discharge_time","operator":"LESS_THAN","referenceField":"admit_time","threshold":null,"condition":null,"defectTemplate":"出院时间早于入院时间","suggestion":"请核实出院时间是否正确"}',
        N'检查出院时间与入院时间的逻辑关系',
        5
    );

    -- 6. 完整性 - 入院记录24h内完成
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'TIMELINESS_002',
        N'首次病程记录8h内完成',
        'TIMELINESS',
        'A',
        '{"ruleCode":"TIMELINESS_002","ruleName":"首次病程记录8h内完成","category":"TIMELINESS","targetField":"raw_data.progress_note.first_time","operator":"HOURS_SINCE","referenceField":"admit_time","threshold":8,"condition":"GREATER_THAN","defectTemplate":"首次病程记录未在8h内完成，实际耗时 {actual} 小时","suggestion":"请在患者入院8小时内完成首次病程记录"}',
        N'检查首次病程记录是否在入院8小时内完成',
        6
    );

    -- 7. 一致性 - 入院诊断与出院诊断冲突（简化校验）
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'CONSISTENCY_001',
        N'入院诊断与出院诊断不匹配',
        'CONSISTENCY',
        'B',
        '{"ruleCode":"CONSISTENCY_001","ruleName":"入院诊断与出院诊断不匹配","category":"CONSISTENCY","targetField":"raw_data.admission_record.admit_diagnosis","operator":"NOT_EQUALS","referenceField":"diagnosis","threshold":null,"condition":null,"defectTemplate":"入院诊断与出院诊断存在差异","suggestion":"请核对该病例的诊断记录是否准确"}',
        N'检查入院诊断和出院诊断是否一致（简化）',
        7
    );

    -- 8. 完整性 - 手术记录不可为空（有手术的病例）
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'COMPLETENESS_004',
        N'有手术病例需有手术记录',
        'COMPLETENESS',
        'B',
        '{"ruleCode":"COMPLETENESS_004","ruleName":"有手术病例需有手术记录","category":"COMPLETENESS","targetField":"raw_data.surgery_record","operator":"IS_NULL","threshold":null,"condition":"raw_data.has_surgery == true","defectTemplate":"该病例有手术记录但未填写手术记录单","suggestion":"请补充手术记录"}',
        N'检查有手术的病例是否填写手术记录',
        8
    );

    -- 9. 完整性 - 出院小结不可为空
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'COMPLETENESS_005',
        N'出院小结不可为空',
        'COMPLETENESS',
        'A',
        '{"ruleCode":"COMPLETENESS_005","ruleName":"出院小结不可为空","category":"COMPLETENESS","targetField":"raw_data.discharge_summary","operator":"IS_NULL","threshold":null,"condition":null,"defectTemplate":"出院小结为空","suggestion":"请填写出院小结"}',
        N'检查出院小结是否存在',
        9
    );

    -- 10. 时效性 - 出院记录24h内完成
    INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level, rule_expression, rule_desc, priority)
    VALUES (
        'TIMELINESS_003',
        N'出院记录24h内完成',
        'TIMELINESS',
        'B',
        '{"ruleCode":"TIMELINESS_003","ruleName":"出院记录24h内完成","category":"TIMELINESS","targetField":"raw_data.discharge_summary.create_time","operator":"HOURS_SINCE","referenceField":"discharge_time","threshold":24,"condition":"GREATER_THAN","defectTemplate":"出院记录未在24h内完成，实际耗时 {actual} 小时","suggestion":"请在患者出院24小时内完成出院记录"}',
        N'检查出院记录是否在出院后24小时内完成',
        10
    );
END
GO

PRINT '初始数据脚本执行完成。';
GO
