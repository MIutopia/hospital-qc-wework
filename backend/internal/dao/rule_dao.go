package dao

import (
	"fmt"

	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// RuleDAO 质控规则数据访问
type RuleDAO struct {
	db *sqlx.DB
}

func NewRuleDAO(db *sqlx.DB) *RuleDAO {
	return &RuleDAO{db: db}
}

// GetEnabledRules 获取全部启用规则（按优先级排序）
func (d *RuleDAO) GetEnabledRules() ([]model.QCRule, error) {
	var rules []model.QCRule
	err := d.db.Select(&rules, `
		SELECT id, rule_code, rule_name, rule_category, rule_level,
		       rule_expression, rule_desc, is_enabled, priority,
		       created_at, updated_at
		FROM qc_rule
		WHERE is_enabled = 1
		ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// GetByID 根据 ID 查询规则
func (d *RuleDAO) GetByID(id int64) (*model.QCRule, error) {
	var r model.QCRule
	err := d.db.Get(&r, `SELECT * FROM qc_rule WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Create 创建规则
func (d *RuleDAO) Create(r *model.QCRule) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO qc_rule (rule_code, rule_name, rule_category, rule_level,
		                     rule_expression, rule_desc, is_enabled, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, r.RuleCode, r.RuleName, r.RuleCategory, r.RuleLevel,
		r.RuleExpression, r.RuleDesc, r.IsEnabled, r.Priority)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Update 更新规则
func (d *RuleDAO) Update(r *model.QCRule) error {
	_, err := d.db.Exec(`
		UPDATE qc_rule
		SET rule_name = ?, rule_category = ?, rule_level = ?,
		    rule_expression = ?, rule_desc = ?,
		    is_enabled = ?, priority = ?,
		    updated_at = GETDATE()
		WHERE id = ?
	`, r.RuleName, r.RuleCategory, r.RuleLevel,
		r.RuleExpression, r.RuleDesc,
		r.IsEnabled, r.Priority, r.ID)
	return err
}

// Delete 删除规则
func (d *RuleDAO) Delete(id int64) error {
	_, err := d.db.Exec(`DELETE FROM qc_rule WHERE id = ?`, id)
	return err
}

// List 分页查询规则
func (d *RuleDAO) List(page, pageSize int) ([]model.QCRule, int, error) {
	var total int
	err := d.db.Get(&total, `SELECT COUNT(*) FROM qc_rule`)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var rules []model.QCRule
	// go-mssqldb 对 OFFSET/FETCH 中的 ? 占位符支持不佳，整数直接内联（page/pageSize 已由 Atoi 校验，无注入风险）
	err = d.db.Select(&rules, fmt.Sprintf(`
		SELECT * FROM qc_rule
		ORDER BY priority ASC, id ASC
		OFFSET %d ROWS FETCH NEXT %d ROWS ONLY
	`, offset, pageSize))
	if err != nil {
		return nil, 0, err
	}

	return rules, total, nil
}
