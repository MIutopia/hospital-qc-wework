package qc

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// RuleDSL 规则 DSL 定义（对应 qc_rule.rule_expression JSON）
type RuleDSL struct {
	RuleCode       string      `json:"ruleCode"`
	RuleName       string      `json:"ruleName"`
	Category       string      `json:"category"`
	TargetField    string      `json:"targetField"`
	Operator       string      `json:"operator"`
	ReferenceField string      `json:"referenceField,omitempty"`
	Threshold      interface{} `json:"threshold,omitempty"`
	Condition      string      `json:"condition,omitempty"`
	DefectTemplate string      `json:"defectTemplate"`
	Suggestion     string      `json:"suggestion"`
}

// Env 规则执行环境（expr 表达式可访问的变量）
type Env struct {
	AdmitTime     time.Time              `expr:"admit_time"`
	DischargeTime *time.Time             `expr:"discharge_time"`
	Diagnosis     string                 `expr:"diagnosis"`
	CaseStatus    string                 `expr:"case_status"`
	RawData       map[string]interface{} `expr:"raw_data"`
	DeptName      string                 `expr:"dept_name"`
	DoctorName    string                 `expr:"doctor_name"`
	PatientGender int                    `expr:"patient_gender"`
	PatientAge    int                    `expr:"patient_age"`
}

// DSLParser DSL 解析器
type DSLParser struct{}

// NewDSLParser 创建 DSL 解析器
func NewDSLParser() *DSLParser {
	return &DSLParser{}
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
//
// 设计说明（2026-08-03 修复，详见交接文档 M3 技术瓶颈表）：
//  1. expr v1.16.9 中 contains/matches 是「中缀运算符」而非函数，
//     必须生成 `str(x) contains "y"` / `str(x) matches "re"` 形式；
//     原先 `contains(x, y)` 函数调用写法无法编译。
//  2. expr 对 map 二级以上路径访问在键缺失时会直接运行报错
//     （cannot fetch x from <nil>），因此 raw_data.* 一律通过自定义
//     get(raw_data, "a.b.c") 安全取值，缺失返回 nil，不中断批次。
//  3. 时间差函数 hoursSince/daysSince 支持 time.Time / *time.Time /
//     字符串（"2006-01-02 15:04:05" 等）；目标字段缺失视为 +Inf，
//     使「N小时内未完成」类规则在文书缺失时正确命中缺陷。
func (p *DSLParser) ToExpr(dsl *RuleDSL) (string, error) {
	switch dsl.Operator {
	case "IS_NULL":
		return fieldExpr(dsl.TargetField) + " == nil", nil
	case "IS_NOT_NULL":
		return fieldExpr(dsl.TargetField) + " != nil", nil
	case "EQUALS":
		return fmt.Sprintf("%s == %v", fieldExpr(dsl.TargetField), formatValue(dsl.Threshold)), nil
	case "NOT_EQUALS":
		return fmt.Sprintf("%s != %v", fieldExpr(dsl.TargetField), formatValue(dsl.Threshold)), nil
	case "GREATER_THAN":
		// 设置了 referenceField 时比较两个字段（如出院时间 > 入院时间，用 after 保证 nil 安全），否则与字面量阈值比较
		if dsl.ReferenceField != "" {
			return fmt.Sprintf("after(%s, %s)", fieldExpr(dsl.TargetField), fieldExpr(dsl.ReferenceField)), nil
		}
		return fmt.Sprintf("%s > %v", numericFieldExpr(dsl.TargetField), formatNumber(dsl.Threshold)), nil
	case "LESS_THAN":
		if dsl.ReferenceField != "" {
			return fmt.Sprintf("before(%s, %s)", fieldExpr(dsl.TargetField), fieldExpr(dsl.ReferenceField)), nil
		}
		return fmt.Sprintf("%s < %v", numericFieldExpr(dsl.TargetField), formatNumber(dsl.Threshold)), nil
	case "HOURS_SINCE":
		return fmt.Sprintf("hoursSince(%s, %s) > %v",
			fieldExpr(dsl.ReferenceField), fieldExpr(dsl.TargetField), formatNumber(dsl.Threshold)), nil
	case "DAYS_SINCE":
		return fmt.Sprintf("daysSince(%s, %s) > %v",
			fieldExpr(dsl.ReferenceField), fieldExpr(dsl.TargetField), formatNumber(dsl.Threshold)), nil
	case "CONTAINS":
		return fmt.Sprintf("str(%s) contains %v", fieldExpr(dsl.TargetField), formatValue(dsl.Threshold)), nil
	case "REGEX":
		return fmt.Sprintf("str(%s) matches %v", fieldExpr(dsl.TargetField), formatValue(dsl.Threshold)), nil
	case "CROSS_CHECK":
		if strings.TrimSpace(dsl.Condition) == "" {
			return "", fmt.Errorf("CROSS_CHECK 规则缺少 condition 表达式")
		}
		return dsl.Condition, nil
	default:
		return "", fmt.Errorf("不支持的运算符: %s", dsl.Operator)
	}
}

// Compile 编译 DSL 为可执行表达式
func (p *DSLParser) Compile(dsl *RuleDSL) (*vm.Program, error) {
	exprStr, err := p.ToExpr(dsl)
	if err != nil {
		return nil, err
	}

	program, err := expr.Compile(exprStr, expr.Env(&Env{}),
		expr.Function("get", getField, func(m map[string]interface{}, path string) any { return nil }),
		expr.Function("str", toStr, func(v any) string { return "" }),
		expr.Function("num", toNum, func(v any) float64 { return 0 }),
		expr.Function("hoursSince", func(params ...any) (any, error) {
			return hoursSince(params[0], params[1]), nil
		}, func(reference, target any) float64 { return 0 }),
		expr.Function("daysSince", func(params ...any) (any, error) {
			return daysSince(params[0], params[1]), nil
		}, func(reference, target any) float64 { return 0 }),
		expr.Function("before", func(params ...any) (any, error) {
			return before(params[0], params[1]), nil
		}, func(target, reference any) bool { return false }),
		expr.Function("after", func(params ...any) (any, error) {
			return after(params[0], params[1]), nil
		}, func(target, reference any) bool { return false }),
	)
	if err != nil {
		return nil, fmt.Errorf("表达式编译失败 [%s]: %s: %w", dsl.RuleCode, exprStr, err)
	}

	return program, nil
}

// Eval 执行表达式求值
func (p *DSLParser) Eval(program *vm.Program, env *Env) (bool, error) {
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

// ----- 字段表达式生成 -----

// fieldExpr 生成字段访问表达式：
//   - 顶层字段（admit_time / diagnosis / patient_age ...）原样使用；
//   - raw_data.* 使用 get(raw_data, "路径") 安全取值（缺失返回 nil）。
func fieldExpr(field string) string {
	if strings.HasPrefix(field, "raw_data.") {
		return fmt.Sprintf("get(raw_data, %q)", strings.TrimPrefix(field, "raw_data."))
	}
	return field
}

// numericFieldExpr 生成数值字段访问表达式：
//   - 顶层数值字段原样使用；
//   - raw_data.* 使用 num(get(...))，缺失/非数值返回 0。
func numericFieldExpr(field string) string {
	if strings.HasPrefix(field, "raw_data.") {
		return fmt.Sprintf("num(get(raw_data, %q))", strings.TrimPrefix(field, "raw_data."))
	}
	return field
}

// ----- 自定义函数 -----

// getField 安全嵌套取值：obj 为 map，path 形如 "a.b.c"，任意层级缺失返回 nil
func getField(params ...any) (any, error) {
	obj, ok := params[0].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	path, ok := params[1].(string)
	if !ok {
		return nil, nil
	}

	var cur any = obj
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, nil
		}
		v, ok := m[part]
		if !ok {
			return nil, nil
		}
		cur = v
	}
	return cur, nil
}

// toStr 任意值转字符串：nil → ""，其他 → fmt 表示
func toStr(params ...any) (any, error) {
	if params[0] == nil {
		return "", nil
	}
	if s, ok := params[0].(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", params[0]), nil
}

// toNum 任意值转数值：nil/非数值 → 0，支持 int/float/数字字符串
func toNum(params ...any) (any, error) {
	switch v := params[0].(type) {
	case nil:
		return 0.0, nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f, nil
		}
		return 0.0, nil
	default:
		return 0.0, nil
	}
}

// hoursSince 计算两个时间字段之间的小时差
// 规则：reference 缺失 → 0（无法判定，不触发）；target 缺失 → +Inf（文书缺失，命中超时）
func hoursSince(reference, target interface{}) float64 {
	ref, ok := toTime(reference)
	if !ok {
		return 0
	}
	tar, ok := toTime(target)
	if !ok {
		return math.Inf(1)
	}
	return tar.Sub(ref).Hours()
}

// daysSince 计算两个时间字段之间的天数差
func daysSince(reference, target interface{}) float64 {
	ref, ok := toTime(reference)
	if !ok {
		return 0
	}
	tar, ok := toTime(target)
	if !ok {
		return math.Inf(1)
	}
	return tar.Sub(ref).Hours() / 24
}

// before 判断 target 是否早于 reference（target 缺失/不可解析 → false）
func before(target, reference interface{}) bool {
	t, ok := toTime(target)
	if !ok {
		return false
	}
	r, ok := toTime(reference)
	if !ok {
		return false
	}
	return t.Before(r)
}

// after 判断 target 是否晚于 reference（target 缺失/不可解析 → false）
func after(target, reference interface{}) bool {
	t, ok := toTime(target)
	if !ok {
		return false
	}
	r, ok := toTime(reference)
	if !ok {
		return false
	}
	return t.After(r)
}

// toTime 将 time.Time / *time.Time / 字符串转换为 time.Time
func toTime(v interface{}) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	case string:
		if strings.TrimSpace(t) == "" {
			return time.Time{}, false
		}
		formats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"2006-01-02",
			"2006/01/02 15:04:05",
			"2006/01/02",
			time.RFC3339,
		}
		for _, f := range formats {
			if tt, err := time.ParseInLocation(f, strings.TrimSpace(t), time.Local); err == nil {
				return tt, true
			}
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}
// ----- 格式化工具 -----

// formatValue 格式化阈值为 expr 字面量：字符串加引号，其他按原样
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case nil:
		return "nil"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// formatNumber 格式化数值阈值：nil → 0
func formatNumber(v interface{}) string {
	if v == nil {
		return "0"
	}
	return fmt.Sprintf("%v", v)
}
