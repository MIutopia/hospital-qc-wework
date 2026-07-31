package model

import "time"

// QCRule 质控规则表
type QCRule struct {
	ID             int64     `db:"id" json:"id"`
	RuleCode       string    `db:"rule_code" json:"ruleCode"`
	RuleName       string    `db:"rule_name" json:"ruleName"`
	RuleCategory   string    `db:"rule_category" json:"ruleCategory"`     // TIMELINESS / COMPLETENESS / LOGIC / CONSISTENCY
	RuleLevel      string    `db:"rule_level" json:"ruleLevel"`           // A(严重) / B(一般) / C(提示)
	RuleExpression string    `db:"rule_expression" json:"ruleExpression"` // JSON DSL, NVARCHAR(MAX)
	RuleDesc       *string   `db:"rule_desc" json:"ruleDesc"`
	IsEnabled      int       `db:"is_enabled" json:"isEnabled"` // 0/1
	Priority       int       `db:"priority" json:"priority"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `db:"updated_at" json:"updatedAt"`
}

// RuleCategory 规则分类常量
const (
	RuleCategoryTimeliness  = "TIMELINESS"
	RuleCategoryCompleteness = "COMPLETENESS"
	RuleCategoryLogic       = "LOGIC"
	RuleCategoryConsistency = "CONSISTENCY"
)

// RuleLevel 规则等级常量
const (
	RuleLevelA = "A" // 严重
	RuleLevelB = "B" // 一般
	RuleLevelC = "C" // 提示
)
