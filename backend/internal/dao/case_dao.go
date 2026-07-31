package dao

import (
	"time"

	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// CaseDAO 病例数据访问
type CaseDAO struct {
	db *sqlx.DB
}

func NewCaseDAO(db *sqlx.DB) *CaseDAO {
	return &CaseDAO{db: db}
}

// GetPendingCases 查询待质控病例（admit_time 近7天）
func (d *CaseDAO) GetPendingCases(limit int) ([]model.InpatientCase, error) {
	var cases []model.InpatientCase
	err := d.db.Select(&cases, `
		SELECT id, case_no, patient_name, patient_gender, patient_age,
		       admit_time, discharge_time, dept_id, dept_name,
		       doctor_id, doctor_name, diagnosis, case_status,
		       raw_data, sync_time, qc_status, qc_time,
		       created_at, updated_at
		FROM inpatient_case
		WHERE qc_status = 'PENDING'
		  AND admit_time >= DATEADD(DAY, -7, GETDATE())
		ORDER BY admit_time ASC
	`)
	if err != nil {
		return nil, err
	}
	return cases, nil
}

// GetByID 根据 ID 查询病例
func (d *CaseDAO) GetByID(id int64) (*model.InpatientCase, error) {
	var c model.InpatientCase
	err := d.db.Get(&c, `SELECT * FROM inpatient_case WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetByCaseNo 根据住院号查询
func (d *CaseDAO) GetByCaseNo(caseNo string) (*model.InpatientCase, error) {
	var c model.InpatientCase
	err := d.db.Get(&c, `SELECT * FROM inpatient_case WHERE case_no = ?`, caseNo)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateQCStatus 更新质控状态
func (d *CaseDAO) UpdateQCStatus(id int64, status string) error {
	_, err := d.db.Exec(`
		UPDATE inpatient_case
		SET qc_status = ?, qc_time = GETDATE(), updated_at = GETDATE()
		WHERE id = ?
	`, status, id)
	return err
}

// UpsertByCaseNo 按住院号 upsert 病例
// 返回 isNew：true=新建，false=更新
func (d *CaseDAO) UpsertByCaseNo(c *model.InpatientCase) (bool, error) {
	// 先查后写：判断是否存在
	var exists int
	err := d.db.Get(&exists, `SELECT COUNT(*) FROM inpatient_case WHERE case_no = ?`, c.CaseNo)
	if err != nil {
		return false, err
	}

	if exists > 0 {
		// 更新
		_, err = d.db.Exec(`
			UPDATE inpatient_case
			SET patient_name = ?, patient_gender = ?, patient_age = ?,
			    admit_time = ?, discharge_time = ?, dept_id = ?, dept_name = ?,
			    doctor_id = ?, doctor_name = ?, diagnosis = ?, case_status = ?,
			    raw_data = ?, sync_time = GETDATE(), updated_at = GETDATE()
			WHERE case_no = ?
		`, c.PatientName, c.PatientGender, c.PatientAge,
			c.AdmitTime, c.DischargeTime, c.DeptID, c.DeptName,
			c.DoctorID, c.DoctorName, c.Diagnosis, c.CaseStatus,
			c.RawData, c.CaseNo)
		if err != nil {
			return false, err
		}
		return false, nil
	}

	// 插入
	_, err = d.db.Exec(`
		INSERT INTO inpatient_case (case_no, patient_name, patient_gender, patient_age,
		                            admit_time, discharge_time, dept_id, dept_name,
		                            doctor_id, doctor_name, diagnosis, case_status,
		                            raw_data, sync_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, GETDATE())
	`, c.CaseNo, c.PatientName, c.PatientGender, c.PatientAge,
		c.AdmitTime, c.DischargeTime, c.DeptID, c.DeptName,
		c.DoctorID, c.DoctorName, c.Diagnosis, c.CaseStatus,
		c.RawData)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetMaxSyncTime 获取最大同步时间（增量断点）
// 无数据时返回空字符串（首次同步取最近30天）
func (d *CaseDAO) GetMaxSyncTime() (string, error) {
	var t *time.Time
	err := d.db.Get(&t, `SELECT MAX(sync_time) FROM inpatient_case`)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "", nil
	}
	return t.Format("2006-01-02 15:04:05"), nil
}

// FindDeptIDByName 按科室名称查找科室 ID（找不到返回 0）
func (d *CaseDAO) FindDeptIDByName(name string) int64 {
	var id int64
	err := d.db.Get(&id, `SELECT id FROM department WHERE dept_name = ?`, name)
	if err != nil {
		return 0
	}
	return id
}

// GetDoctorCases 获取医生的病例列表
func (d *CaseDAO) GetDoctorCases(doctorID int64, status string, page, pageSize int) ([]model.InpatientCase, int, error) {
	var total int
	err := d.db.Get(&total, `
		SELECT COUNT(*) FROM inpatient_case
		WHERE doctor_id = ?
		  AND (qc_status = ? OR ? = '')
	`, doctorID, status, status)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var cases []model.InpatientCase
	err = d.db.Select(&cases, `
		SELECT id, case_no, patient_name, patient_gender, patient_age,
		       admit_time, discharge_time, dept_id, dept_name,
		       doctor_id, doctor_name, diagnosis,
		       qc_status, qc_time, created_at
		FROM inpatient_case
		WHERE doctor_id = ?
		  AND (qc_status = ? OR ? = '')
		ORDER BY qc_time DESC
		OFFSET ? ROWS FETCH NEXT ? ROWS ONLY
	`, doctorID, status, status, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	return cases, total, nil
}
