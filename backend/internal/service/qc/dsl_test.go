package qc

import (
	"testing"
)

// TestDSLParser_Parse 验证 DSL JSON 解析
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

// TestDSLParser_ToExpr 验证 DSL 转 expr 表达式
func TestDSLParser_ToExpr(t *testing.T) {
	parser := NewDSLParser()

	tests := []struct {
		name    string
		dsl     RuleDSL
		wantErr bool
	}{
		{
			name: "IS_NULL → target == nil",
			dsl: RuleDSL{
				RuleCode:    "TEST_001",
				TargetField: "diagnosis",
				Operator:    "IS_NULL",
			},
			wantErr: false,
		},
		{
			name: "HOURS_SINCE → hoursSince(...)",
			dsl: RuleDSL{
				RuleCode:       "TEST_002",
				TargetField:    "raw_data.admission_record.create_time",
				Operator:       "HOURS_SINCE",
				ReferenceField: "admit_time",
				Threshold:      24,
			},
			wantErr: false,
		},
		{
			name: "不支持的运算符",
			dsl: RuleDSL{
				RuleCode:    "TEST_003",
				TargetField: "field",
				Operator:    "UNKNOWN_OP",
			},
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
			if !tt.wantErr && exprStr == "" {
				t.Error("ToExpr() returned empty string")
			}
		})
	}
}

// TestDSLParser_Compile 验证表达式编译（核心路径）
func TestDSLParser_Compile(t *testing.T) {
	parser := NewDSLParser()

	dsl := RuleDSL{
		RuleCode:    "COMPLETENESS_001",
		RuleName:    "主诉不可为空",
		Category:    "COMPLETENESS",
		TargetField: "diagnosis",
		Operator:    "IS_NULL",
		DefectTemplate: "诊断为空",
		Suggestion:     "请填写诊断",
	}

	program, err := parser.Compile(&dsl)
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
	if program == nil {
		t.Fatal("Compile() returned nil program")
	}
}

// TestDSLParser_Eval 验证表达式求值
func TestDSLParser_Eval(t *testing.T) {
	parser := NewDSLParser()

	t.Run("IS_NULL 检测 raw_data 中缺失的字段", func(t *testing.T) {
		// IS_NULL 在 raw_data map 中检测 nil 值
		dsl := RuleDSL{
			RuleCode:    "COMPLETENESS_001",
			TargetField: "raw_data.admission_record",
			Operator:    "IS_NULL",
		}
		program, err := parser.Compile(&dsl)
		if err != nil {
			t.Fatalf("Compile() error: %v", err)
		}

		// raw_data 中不存在该键 → map 返回 nil → 命中
		env := Env{RawData: map[string]interface{}{}}
		result, err := parser.Eval(program, &env)
		if err != nil {
			t.Errorf("Eval() error: %v", err)
		}
		if !result {
			t.Error("IS_NULL 应命中：raw_data 中不存在 admission_record")
		}

		// raw_data 中存在该键 → 不命中
		env2 := Env{RawData: map[string]interface{}{
			"admission_record": map[string]interface{}{"complaint": "头痛3天"},
		}}
		result2, err := parser.Eval(program, &env2)
		if err != nil {
			t.Errorf("Eval() error: %v", err)
		}
		if result2 {
			t.Error("IS_NULL 不应命中：raw_data 中存在 admission_record")
		}
	})

	t.Run("EQUALS 字符串匹配", func(t *testing.T) {
		dsl := RuleDSL{
			RuleCode:    "TEST_EQ",
			TargetField: "diagnosis",
			Operator:    "EQUALS",
			Threshold:   "高血压",
		}
		program, err := parser.Compile(&dsl)
		if err != nil {
			t.Fatalf("Compile() error: %v", err)
		}

		env := Env{Diagnosis: "高血压"}
		result, err := parser.Eval(program, &env)
		if err != nil {
			t.Errorf("Eval() error: %v", err)
		}
		if !result {
			t.Error("EQUALS 应命中：诊断与阈值匹配")
		}
	})

	t.Run("GREATER_THAN 年龄阈值", func(t *testing.T) {
		dsl := RuleDSL{
			RuleCode:    "TEST_GT",
			TargetField: "patient_age",
			Operator:    "GREATER_THAN",
			Threshold:   65,
		}
		program, err := parser.Compile(&dsl)
		if err != nil {
			t.Fatalf("Compile() error: %v", err)
		}

		// 年龄 70 > 65 → 命中
		env := Env{PatientAge: 70}
		result, err := parser.Eval(program, &env)
		if err != nil {
			t.Errorf("Eval() error: %v", err)
		}
		if !result {
			t.Error("GREATER_THAN 应命中：70 > 65")
		}
	})

	// 注：CONTAINS 运算符与 expr v1.16.9 内置 contains 关键字存在兼容问题，
	// 已在项目早期记录（见交接文档 M3 技术瓶颈）。expr 原生支持 contains()，
	// M3 阶段将移除自定义 contains 函数注册，改为直接使用 expr 内置实现。
}

// BenchmarkDSLParser_Compile 基准：表达式编译性能
func BenchmarkDSLParser_Compile(b *testing.B) {
	parser := NewDSLParser()
	dsl := RuleDSL{
		RuleCode:    "TIMELINESS_001",
		TargetField: "raw_data.admission_record.create_time",
		Operator:    "HOURS_SINCE",
		ReferenceField: "admit_time",
		Threshold:      24,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Compile(&dsl)
	}
}
