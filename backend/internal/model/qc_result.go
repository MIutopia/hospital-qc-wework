package model

import "time"

// QCResult 质控结果表
type QCResult struct {
	ID             int64     `db:"id" json:"id"`
	CaseID         int64     `db:"case_id" json:"caseId"`
	RuleID         int64     `db:"rule_id" json:"ruleId"`
	IsDefect       int       `db:"is_defect" json:"isDefect"` // 0:否 1:是
	DefectDetail   *string   `db:"defect_detail" json:"defectDetail"`
	DefectLocation *string   `db:"defect_location" json:"defectLocation"`
	Suggestion     *string   `db:"suggestion" json:"suggestion"`
	QCBatchID      *string   `db:"qc_batch_id" json:"qcBatchId"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt"`
}

// DefectSummary 缺陷汇总（用于API响应）
type DefectSummary struct {
	Total  int `json:"total"`
	LevelA int `json:"levelA"`
	LevelB int `json:"levelB"`
	LevelC int `json:"levelC"`
}

// DefectItem 缺陷项（用于API响应）
type DefectItem struct {
	ID             int64  `json:"id"`
	RuleName       string `json:"ruleName"`
	RuleLevel      string `json:"ruleLevel"`
	DefectDetail   string `json:"defectDetail"`
	DefectLocation string `json:"defectLocation"`
	Suggestion     string `json:"suggestion"`
}
