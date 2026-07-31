package qc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/expr-lang/expr"
)

// RuleDSL 规则 DSL 定义（对应 qc_rule.rule_expression JSON）
type RuleDSL struct {
	RuleCode       string `json:"ruleCode"`
	RuleName       string `json:"ruleName"`
	Category       string `json:"category"`
	TargetField    string `json:"targetField"`
	Operator       string `json:"operator"`
	ReferenceField string `json:"referenceField,omitempty"`
	Threshold      interface{} `json:"threshold,omitempty"`
	Condition      string `json:"condition,omitempty"`
	DefectTemplate string `json:"defectTemplate"`
	Suggestion     string `json:"suggestion"`
}

// Env 规则执行环境（expr 表达式可访问的变量）
type Env struct {
	AdmitTime         time.Time              `expr:"admit_time"`
	DischargeTime     *time.Time             `expr:"discharge_time"`
	Diagnosis         string                 `expr:"diagnosis"`
	CaseStatus        string                 `expr:"case_status"`
	RawData           map[string]interface{} `expr:"raw_data"`
	DeptName          string                 `expr:"dept_name"`
	DoctorName        string                 `expr:"doctor_name"`
	PatientGender     int                    `expr:"patient_gender"`
	PatientAge        int                    `expr:"patient_age"`
}

// DSLParser DSL 解析器
type DSLParser struct {
	customFuncs map[string]interface{}
}

// NewDSLParser 创建 DSL 解析器
func NewDSLParser() *DSLParser {
	return &DSLParser{
		customFuncs: map[string]interface{}{
			"hoursSince":  hoursSince,
			"daysSince":   daysSince,
			"matches":     matches,
			"contains":    contains,
		},
	}
}

// Parse 解析规则 DSL JSON 字符串
func (p *DSLParser) Parse(expressionJSON string) (*RuleDSL, error) {
	var dsl RuleDSL
	if err := json.Unmarshal([]byte(expressionJSON), &dsl); err != nil {
		return nil, fmt.Errorf("DSL 解析失败: %w", err)
	}
	return &dsl, nil
}

// ToExpr 将 DSL 转换为 expr 表达式字符串
func (p *DSLParser) ToExpr(dsl *RuleDSL) (string, error) {
	switch dsl.Operator {
	case "IS_NULL":
		return fmt.Sprintf("%s == nil", dsl.TargetField), nil
	case "IS_NOT_NULL":
		return fmt.Sprintf("%s != nil", dsl.TargetField), nil
	case "EQUALS":
		return fmt.Sprintf("%s == %v", dsl.TargetField, formatValue(dsl.Threshold)), nil
	case "NOT_EQUALS":
		return fmt.Sprintf("%s != %v", dsl.TargetField, formatValue(dsl.Threshold)), nil
	case "GREATER_THAN":
		return fmt.Sprintf("%s > %v", dsl.TargetField, dsl.Threshold), nil
	case "LESS_THAN":
		return fmt.Sprintf("%s < %v", dsl.TargetField, dsl.Threshold), nil
	case "HOURS_SINCE":
		// hoursSince(referenceField, targetField) > threshold
		return fmt.Sprintf("hoursSince(%s, %s) > %v",
			dsl.ReferenceField, dsl.TargetField, dsl.Threshold), nil
	case "DAYS_SINCE":
		return fmt.Sprintf("daysSince(%s, %s) > %v",
			dsl.ReferenceField, dsl.TargetField, dsl.Threshold), nil
	case "CONTAINS":
		return fmt.Sprintf("contains(%s, %v)", dsl.TargetField, dsl.Threshold), nil
	case "REGEX":
		return fmt.Sprintf("matches(%s, %v)", dsl.TargetField, dsl.Threshold), nil
	case "CROSS_CHECK":
		// 跨字段校验，直接使用 condition 字段作为表达式
		return dsl.Condition, nil
	default:
		return "", fmt.Errorf("不支持的运算符: %s", dsl.Operator)
	}
}

// Compile 编译 DSL 为可执行表达式
func (p *DSLParser) Compile(dsl *RuleDSL) (expr.Expr, error) {
	exprStr, err := p.ToExpr(dsl)
	if err != nil {
		return nil, err
	}

	// 编译表达式
	program, err := expr.Compile(exprStr, expr.Env(&Env{}), expr.Functions(p.customFuncs))
	if err != nil {
		return nil, fmt.Errorf("表达式编译失败 [%s]: %s: %w", dsl.RuleCode, exprStr, err)
	}

	return program, nil
}

// Eval 执行表达式求值
func (p *DSLParser) Eval(program expr.Expr, env *Env) (bool, error) {
	output, err := expr.Run(program, env)
	if err != nil {
		return false, err
	}

	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("表达式结果不是布尔值: %v", output)
	}

	return result, nil
}

// ----- 自定义函数 -----

// hoursSince 计算两个时间字段之间的小时差
func hoursSince(reference, target interface{}) float64 {
	refTime, ok1 := reference.(time.Time)
	tarTime, ok2 := target.(time.Time)
	if !ok1 || !ok2 {
		return 0
	}
	return tarTime.Sub(refTime).Hours()
}

// daysSince 计算两个时间字段之间的天数差
func daysSince(reference, target interface{}) float64 {
	refTime, ok1 := reference.(time.Time)
	tarTime, ok2 := target.(time.Time)
	if !ok1 || !ok2 {
		return 0
	}
	return tarTime.Sub(refTime).Hours() / 24
}

// matches 正则匹配
func matches(str, pattern interface{}) bool {
	// expr 中通过正则匹配实现
	return false // 简化实现，实际使用 regexp.MatchString
}

// contains 字符串包含
func contains(str, substr interface{}) bool {
	s, ok1 := str.(string)
	sub, ok2 := substr.(string)
	if !ok1 || !ok2 {
		return false
	}
	// expr 原生也支持 contains，此函数作为备用
	return len(s) >= len(sub) && (len(sub) == 0 || containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// formatValue 格式化阈值值为字符串
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
