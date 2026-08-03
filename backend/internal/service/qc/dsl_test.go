package qc

import (
	"testing"
	"time"

	"hospital-qc-wework/internal/model"
)

// buildTestEnv 构造测试用执行环境
func buildTestEnv() *Env {
	return &Env{
		AdmitTime:     time.Date(2026, 7, 28, 10, 30, 0, 0, time.Local),
		Diagnosis:     "高血压病2级",
		CaseStatus:    "ACTIVE",
		DeptName:      "心内科",
		DoctorName:    "李明",
		PatientGender: 1,
		PatientAge:    70,
		RawData: map[string]interface{}{
			"admission_record": map[string]interface{}{
				"complaint":     "头痛3天",
				"create_time":   "2026-07-28 15:04:05", // 入院 10:30，记录 15:04 → 约 4.6h
				"past_history":  "糖尿病史",
				"physical_exam": "神清，心肺听诊未见明显异常",
			},
			"has_surgery": false,
		},
	}
}

// runRule 编译并执行单条 DSL 规则
func runRule(t *testing.T, dsl RuleDSL, env *Env) bool {
	t.Helper()
	parser := NewDSLParser()
	program, err := parser.Compile(&dsl)
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	result, err := parser.Eval(program, env)
	if err != nil {
		t.Fatalf("Eval() error: %v", err)
	}
	return result
}

func TestDSLParser_Parse(t *testing.T) {
	parser := NewDSLParser()

	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "IS_NULL 规则",
			json:    `{"ruleCode":"COMPLETENESS_001","ruleName":"主诉不可为空","category":"COMPLETENESS","targetField":"raw_data.admission_record.complaint","operator":"IS_NULL","defectTemplate":"入院记录中主诉字段为空","suggestion":"请补充患者主诉内容"}`,
			wantErr: false,
		},
		{
			name:    "HOURS_SINCE 规则",
			json:    `{"ruleCode":"TIMELINESS_001","ruleName":"入院记录24h内完成","category":"TIMELINESS","targetField":"raw_data.admission_record.create_time","operator":"HOURS_SINCE","referenceField":"admit_time","threshold":24,"condition":"GREATER_THAN","defectTemplate":"入院记录未在规定24h内完成","suggestion":"请在患者入院24小时内完成入院记录书写"}`,
			wantErr: false,
		},
		{
			name:    "无效 JSON",
			json:    `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsl, err := parser.Parse(tt.json)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && dsl == nil {
				t.Error("Parse() returned nil DSL")
			}
		})
	}
}

func TestDSLParser_ToExpr(t *testing.T) {
	parser := NewDSLParser()

	tests := []struct {
		name    string
		dsl     RuleDSL
		want    string
		wantErr bool
	}{
		{
			name: "IS_NULL 顶层字段",
			dsl:  RuleDSL{TargetField: "diagnosis", Operator: "IS_NULL"},
			want: "diagnosis == nil",
		},
		{
			name: "IS_NULL raw_data 字段 → get 安全取值",
			dsl:  RuleDSL{TargetField: "raw_data.admission_record.complaint", Operator: "IS_NULL"},
			want: `get(raw_data, "admission_record.complaint") == nil`,
		},
		{
			name: "IS_NOT_NULL",
			dsl:  RuleDSL{TargetField: "diagnosis", Operator: "IS_NOT_NULL"},
			want: "diagnosis != nil",
		},
		{
			name:    "EQUALS",
			dsl:     RuleDSL{TargetField: "diagnosis", Operator: "EQUALS", Threshold: "高血压"},
			want:    `diagnosis == "高血压"`,
		},
		{
			name:    "NOT_EQUALS",
			dsl:     RuleDSL{TargetField: "diagnosis", Operator: "NOT_EQUALS", Threshold: "高血压"},
			want:    `diagnosis != "高血压"`,
		},
		{
			name:    "GREATER_THAN 顶层数值",
			dsl:     RuleDSL{TargetField: "patient_age", Operator: "GREATER_THAN", Threshold: 65},
			want:    "patient_age > 65",
		},
		{
			name:    "GREATER_THAN raw_data 数值 → num(get(...))",
			dsl:     RuleDSL{TargetField: "raw_data.vitals.heart_rate", Operator: "GREATER_THAN", Threshold: 100},
			want:    `num(get(raw_data, "vitals.heart_rate")) > 100`,
		},
		{
			name:    "LESS_THAN 与阈值比较",
			dsl:     RuleDSL{TargetField: "patient_age", Operator: "LESS_THAN", Threshold: 18},
			want:    "patient_age < 18",
		},
		{
			name:    "LESS_THAN 字段间比较（出院时间早于入院时间）",
			dsl:     RuleDSL{TargetField: "discharge_time", Operator: "LESS_THAN", ReferenceField: "admit_time"},
			want:    "before(discharge_time, admit_time)",
		},
		{
			name:    "HOURS_SINCE",
			dsl:     RuleDSL{TargetField: "raw_data.admission_record.create_time", Operator: "HOURS_SINCE", ReferenceField: "admit_time", Threshold: 24},
			want:    `hoursSince(admit_time, get(raw_data, "admission_record.create_time")) > 24`,
		},
		{
			name:    "DAYS_SINCE",
			dsl:     RuleDSL{TargetField: "raw_data.admission_record.create_time", Operator: "DAYS_SINCE", ReferenceField: "admit_time", Threshold: 1},
			want:    `daysSince(admit_time, get(raw_data, "admission_record.create_time")) > 1`,
		},
		{
			name:    "CONTAINS → expr 中缀语法",
			dsl:     RuleDSL{TargetField: "raw_data.admission_record.complaint", Operator: "CONTAINS", Threshold: "头痛"},
			want:    `str(get(raw_data, "admission_record.complaint")) contains "头痛"`,
		},
		{
			name:    "REGEX → expr 中缀语法",
			dsl:     RuleDSL{TargetField: "raw_data.admission_record.complaint", Operator: "REGEX", Threshold: "^头痛"},
			want:    `str(get(raw_data, "admission_record.complaint")) matches "^头痛"`,
		},
		{
			name:    "CROSS_CHECK 使用 condition",
			dsl:     RuleDSL{TargetField: "raw_data.surgery_record", Operator: "CROSS_CHECK", Condition: "raw_data.has_surgery == true"},
			want:    "raw_data.has_surgery == true",
		},
		{
			name:    "CROSS_CHECK 缺少 condition",
			dsl:     RuleDSL{Operator: "CROSS_CHECK"},
			wantErr: true,
		},
		{
			name:    "不支持的运算符",
			dsl:     RuleDSL{TargetField: "field", Operator: "UNKNOWN_OP"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprStr, err := parser.ToExpr(&tt.dsl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToExpr() error = %v, wantErr %v, exprStr = %s", err, tt.wantErr, exprStr)
				return
			}
			if !tt.wantErr && exprStr != tt.want {
				t.Errorf("ToExpr() = %q, want %q", exprStr, tt.want)
			}
		})
	}
}

// TestDSLParser_Eval_Operators 覆盖 M3 交付要求：8 类运算符（含子类）逐条求值验证
func TestDSLParser_Eval_Operators(t *testing.T) {
	t.Run("IS_NULL raw_data 键缺失 → 命中", func(t *testing.T) {
		env := buildTestEnv()
		env.RawData = map[string]interface{}{} // 整体缺失
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.admission_record",
			Operator:    "IS_NULL",
		}, env)
		if !result {
			t.Error("IS_NULL 应命中：raw_data 中不存在 admission_record")
		}
	})

	t.Run("IS_NULL raw_data 深层键缺失 → 命中（修复前会运行报错）", func(t *testing.T) {
		env := buildTestEnv()
		env.RawData = map[string]interface{}{"admission_record": map[string]interface{}{}}
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.admission_record.complaint",
			Operator:    "IS_NULL",
		}, env)
		if !result {
			t.Error("IS_NULL 应命中：admission_record.complaint 不存在")
		}
	})

	t.Run("IS_NULL raw_data 键存在 → 不命中", func(t *testing.T) {
		env := buildTestEnv()
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.admission_record.complaint",
			Operator:    "IS_NULL",
		}, env)
		if result {
			t.Error("IS_NULL 不应命中：complaint 已填写")
		}
	})

	t.Run("IS_NOT_NULL", func(t *testing.T) {
		env := buildTestEnv()
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.admission_record.complaint",
			Operator:    "IS_NOT_NULL",
		}, env)
		if !result {
			t.Error("IS_NOT_NULL 应命中：complaint 已填写")
		}
	})

	t.Run("EQUALS 字符串匹配", func(t *testing.T) {
		env := buildTestEnv()
		result := runRule(t, RuleDSL{
			TargetField: "diagnosis",
			Operator:    "EQUALS",
			Threshold:   "高血压病2级",
		}, env)
		if !result {
			t.Error("EQUALS 应命中：诊断完全匹配")
		}
	})

	t.Run("NOT_EQUALS", func(t *testing.T) {
		env := buildTestEnv()
		result := runRule(t, RuleDSL{
			TargetField: "diagnosis",
			Operator:    "NOT_EQUALS",
			Threshold:   "冠心病",
		}, env)
		if !result {
			t.Error("NOT_EQUALS 应命中：诊断不等于冠心病")
		}
	})

	t.Run("GREATER_THAN 年龄阈值", func(t *testing.T) {
		env := buildTestEnv() // age=70
		result := runRule(t, RuleDSL{
			TargetField: "patient_age",
			Operator:    "GREATER_THAN",
			Threshold:   65,
		}, env)
		if !result {
			t.Error("GREATER_THAN 应命中：70 > 65")
		}
	})

	t.Run("LESS_THAN 年龄阈值", func(t *testing.T) {
		env := buildTestEnv() // age=70
		result := runRule(t, RuleDSL{
			TargetField: "patient_age",
			Operator:    "LESS_THAN",
			Threshold:   18,
		}, env)
		if result {
			t.Error("LESS_THAN 不应命中：70 < 18 为假")
		}
	})

	t.Run("HOURS_SINCE 记录在时限内 → 不命中", func(t *testing.T) {
		env := buildTestEnv() // 记录 15:04，入院 10:30 → 约 4.6h < 24h
		result := runRule(t, RuleDSL{
			TargetField:    "raw_data.admission_record.create_time",
			Operator:       "HOURS_SINCE",
			ReferenceField: "admit_time",
			Threshold:      24,
		}, env)
		if result {
			t.Error("HOURS_SINCE 不应命中：4.6h 在 24h 内")
		}
	})

	t.Run("HOURS_SINCE 文书缺失 → 命中（+Inf）", func(t *testing.T) {
		env := buildTestEnv()
		env.RawData = map[string]interface{}{} // 入院记录整体缺失
		result := runRule(t, RuleDSL{
			TargetField:    "raw_data.admission_record.create_time",
			Operator:       "HOURS_SINCE",
			ReferenceField: "admit_time",
			Threshold:      24,
		}, env)
		if !result {
			t.Error("HOURS_SINCE 应命中：文书缺失视为超时")
		}
	})

	t.Run("DAYS_SINCE 超时命中", func(t *testing.T) {
		env := buildTestEnv()
		env.RawData = map[string]interface{}{
			"discharge_summary": map[string]interface{}{
				"create_time": "2026-08-02 10:00:00", // 距入院 2026-07-28 超过 1 天
			},
		}
		result := runRule(t, RuleDSL{
			TargetField:    "raw_data.discharge_summary.create_time",
			Operator:       "DAYS_SINCE",
			ReferenceField: "admit_time",
			Threshold:      1,
		}, env)
		if !result {
			t.Error("DAYS_SINCE 应命中：超过 1 天")
		}
	})

	t.Run("CONTAINS 命中", func(t *testing.T) {
		env := buildTestEnv()
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.admission_record.complaint",
			Operator:    "CONTAINS",
			Threshold:   "头痛",
		}, env)
		if !result {
			t.Error("CONTAINS 应命中：主诉包含「头痛」")
		}
	})

	t.Run("CONTAINS 字段缺失 → 不命中且不报错", func(t *testing.T) {
		env := buildTestEnv()
		env.RawData = map[string]interface{}{}
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.admission_record.complaint",
			Operator:    "CONTAINS",
			Threshold:   "头痛",
		}, env)
		if result {
			t.Error("CONTAINS 不应命中：主诉缺失")
		}
	})

	t.Run("REGEX 命中", func(t *testing.T) {
		env := buildTestEnv()
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.admission_record.complaint",
			Operator:    "REGEX",
			Threshold:   "^头痛",
		}, env)
		if !result {
			t.Error("REGEX 应命中：主诉以「头痛」开头")
		}
	})

	t.Run("CROSS_CHECK 组合条件", func(t *testing.T) {
		env := buildTestEnv() // has_surgery=false
		result := runRule(t, RuleDSL{
			TargetField: "raw_data.surgery_record",
			Operator:    "CROSS_CHECK",
			Condition:   "raw_data.has_surgery == true && raw_data.surgery_record == nil",
		}, env)
		if result {
			t.Error("CROSS_CHECK 不应命中：无手术")
		}

		env.RawData = map[string]interface{}{"has_surgery": true}
		result = runRule(t, RuleDSL{
			TargetField: "raw_data.surgery_record",
			Operator:    "CROSS_CHECK",
			Condition:   "raw_data.has_surgery == true && raw_data.surgery_record == nil",
		}, env)
		if !result {
			t.Error("CROSS_CHECK 应命中：有手术但无手术记录")
		}
	})
}

// TestDSLParser_Eval_FromSeedRules 用 sql/002_init_data.sql 中的真实规则 JSON 验证端到端
func TestDSLParser_Eval_FromSeedRules(t *testing.T) {
	parser := NewDSLParser()

	seedRules := map[string]string{
		"TIMELINESS_001": `{"ruleCode":"TIMELINESS_001","ruleName":"入院记录24h内完成","category":"TIMELINESS","targetField":"raw_data.admission_record.create_time","operator":"HOURS_SINCE","referenceField":"admit_time","threshold":24,"condition":"GREATER_THAN","defectTemplate":"入院记录未在规定24h内完成","suggestion":"请在患者入院24小时内完成入院记录书写"}`,
		"COMPLETENESS_001": `{"ruleCode":"COMPLETENESS_001","ruleName":"主诉不可为空","category":"COMPLETENESS","targetField":"raw_data.admission_record.complaint","operator":"IS_NULL","threshold":null,"condition":null,"defectTemplate":"入院记录中主诉字段为空","suggestion":"请补充患者主诉内容"}`,
		"LOGIC_001":       `{"ruleCode":"LOGIC_001","ruleName":"出院时间不可早于入院时间","category":"LOGIC","targetField":"discharge_time","operator":"LESS_THAN","referenceField":"admit_time","threshold":null,"condition":null,"defectTemplate":"出院时间早于入院时间","suggestion":"请核实出院时间是否正确"}`,
	}

	for code, jsonStr := range seedRules {
		t.Run(code, func(t *testing.T) {
			dsl, err := parser.Parse(jsonStr)
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			program, err := parser.Compile(dsl)
			if err != nil {
				t.Fatalf("Compile() error: %v", err)
			}
			if _, err := parser.Eval(program, buildTestEnv()); err != nil {
				t.Fatalf("Eval() error: %v", err)
			}
		})
	}

	// COMPLETENESS_001：病例无主诉时命中缺陷
	t.Run("COMPLETENESS_001_命中", func(t *testing.T) {
		dsl, _ := parser.Parse(seedRules["COMPLETENESS_001"])
		program, err := parser.Compile(dsl)
		if err != nil {
			t.Fatalf("Compile() error: %v", err)
		}
		env := buildTestEnv()
		env.RawData = map[string]interface{}{}
		result, err := parser.Eval(program, env)
		if err != nil {
			t.Fatalf("Eval() error: %v", err)
		}
		if !result {
			t.Error("主诉缺失应命中缺陷")
		}
	})
}

// TestBuildEnvFromCase 验证从 InpatientCase（含 raw_data JSON）构造 Env
func TestBuildEnvFromCase(t *testing.T) {
	c := makeCaseWithRawData(t, `{"admission_record":{"complaint":"腹痛2天","create_time":"2026-07-28 12:00:00"}}`)
	env := buildEnv(*c)

	if env.RawData["admission_record"].(map[string]interface{})["complaint"] != "腹痛2天" {
		t.Error("raw_data 未正确解析到 Env.RawData")
	}

	// 规则：主诉非空 → 不命中；HOURS_SINCE 在时限内 → 不命中
	parser := NewDSLParser()

	dsl := RuleDSL{TargetField: "raw_data.admission_record.complaint", Operator: "IS_NULL"}
	program, err := parser.Compile(&dsl)
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	result, err := parser.Eval(program, env)
	if err != nil || result {
		t.Errorf("主诉非空应不命中: result=%v err=%v", result, err)
	}
}

// makeCaseWithRawData 构造带 raw_data 的病例（JSON 编码，模拟入库形态）
func makeCaseWithRawData(t *testing.T, rawJSON string) *model.InpatientCase {
	t.Helper()
	raw := rawJSON
	admit := time.Date(2026, 7, 28, 10, 30, 0, 0, time.Local)
	return &model.InpatientCase{
		ID:          1,
		CaseNo:      "ZY202607001",
		PatientName: "张**",
		AdmitTime:   admit,
		QCStatus:    "PENDING",
		RawData:     &raw,
	}
}
