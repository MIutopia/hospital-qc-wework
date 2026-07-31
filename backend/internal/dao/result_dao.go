package dao

import (
	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// ResultDAO 质控结果数据访问
type ResultDAO struct {
	db *sqlx.DB
}

func NewResultDAO(db *sqlx.DB) *ResultDAO {
	return &ResultDAO{db: db}
}

// BatchCreate 批量插入质控结果
func (d *ResultDAO) BatchCreate(results []model.QCResult) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO qc_result (case_id, rule_id, is_defect, defect_detail,
		                       defect_location, suggestion, qc_batch_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range results {
		_, err = stmt.Exec(r.CaseID, r.RuleID, r.IsDefect,
			r.DefectDetail, r.DefectLocation, r.Suggestion, r.QCBatchID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetByCaseID 获取病例的质控结果
func (d *ResultDAO) GetByCaseID(caseID int64) ([]model.QCResult, error) {
	var results []model.QCResult
	err := d.db.Select(&results, `
		SELECT id, case_id, rule_id, is_defect, defect_detail,
		       defect_location, suggestion, qc_batch_id, created_at
		FROM qc_result
		WHERE case_id = ?
		ORDER BY id ASC
	`, caseID)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// GetDefectSummary 获取病例的缺陷汇总
func (d *ResultDAO) GetDefectSummary(caseID int64) (*model.DefectSummary, error) {
	var summary model.DefectSummary
	err := d.db.Get(&summary, `
		SELECT
			COUNT(*) AS total,
			SUM(CASE WHEN qr.rule_level = 'A' THEN 1 ELSE 0 END) AS levelA,
			SUM(CASE WHEN qr.rule_level = 'B' THEN 1 ELSE 0 END) AS levelB,
			SUM(CASE WHEN qr.rule_level = 'C' THEN 1 ELSE 0 END) AS levelC
		FROM qc_result r
		JOIN qc_rule qr ON r.rule_id = qr.id
		WHERE r.case_id = ? AND r.is_defect = 1
	`, caseID)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}
