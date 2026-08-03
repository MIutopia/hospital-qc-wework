package dao

import (
	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// ConfirmDAO 确认整改记录数据访问
type ConfirmDAO struct {
	db *sqlx.DB
}

func NewConfirmDAO(db *sqlx.DB) *ConfirmDAO {
	return &ConfirmDAO{db: db}
}

// Upsert 按 (case_id, doctor_id) 幂等确认整改
// 同一医生对同一病例重复确认只更新一次（覆盖缺陷列表/备注/确认时间）
func (d *ConfirmDAO) Upsert(caseID, doctorID int64, defectIDs *string, note *string) error {
	_, err := d.db.Exec(`
		MERGE qc_confirm AS target
		USING (SELECT ? AS case_id, ? AS doctor_id) AS source
		ON target.case_id = source.case_id AND target.doctor_id = source.doctor_id
		WHEN MATCHED THEN
			UPDATE SET defect_ids = ?, confirm_status = 'CONFIRMED',
			           confirm_note = ?, confirmed_at = GETDATE(), updated_at = GETDATE()
		WHEN NOT MATCHED THEN
			INSERT (case_id, doctor_id, defect_ids, confirm_status, confirm_note, confirmed_at)
			VALUES (?, ?, ?, 'CONFIRMED', ?, GETDATE())
	`, caseID, doctorID, defectIDs, note, caseID, doctorID, defectIDs, note)
	return err
}

// GetByCase 查询病例最近的确认记录
func (d *ConfirmDAO) GetByCase(caseID int64) (*model.QCConfirm, error) {
	var c model.QCConfirm
	err := d.db.Get(&c, `
		SELECT id, case_id, doctor_id, defect_ids, confirm_status,
		       confirm_note, confirmed_at, created_at, updated_at
		FROM qc_confirm
		WHERE case_id = ?
		ORDER BY id DESC
	`, caseID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CountConfirmedByDeptDate 统计某科室某质控日期的已确认病例数（去重）
func (d *ConfirmDAO) CountConfirmedByDeptDate(deptID int64, date string) (int, error) {
	var n int
	err := d.db.Get(&n, `
		SELECT COUNT(DISTINCT qcf.case_id)
		FROM qc_confirm qcf
		JOIN inpatient_case c ON c.id = qcf.case_id
		WHERE c.dept_id = ?
		  AND CONVERT(date, c.qc_time) = ?
		  AND qcf.confirm_status = 'CONFIRMED'
	`, deptID, date)
	if err != nil {
		return 0, err
	}
	return n, nil
}
