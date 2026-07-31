package dao

import (
	"fmt"

	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// DoctorDAO 医生映射数据访问
type DoctorDAO struct {
	db *sqlx.DB
}

func NewDoctorDAO(db *sqlx.DB) *DoctorDAO {
	return &DoctorDAO{db: db}
}

// GetByUserID 根据企业微信 userid 查询医生
func (d *DoctorDAO) GetByUserID(userID string) (*model.DoctorWeWork, error) {
	var doc model.DoctorWeWork
	err := d.db.Get(&doc, `SELECT * FROM doctor_wework WHERE wework_userid = ? AND is_active = 1`, userID)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// GetByDoctorID 根据医生 ID 查询
func (d *DoctorDAO) GetByDoctorID(doctorID int64) (*model.DoctorWeWork, error) {
	var doc model.DoctorWeWork
	err := d.db.Get(&doc, `SELECT * FROM doctor_wework WHERE doctor_id = ? AND is_active = 1`, doctorID)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// FindIDByName 按医生姓名查找 doctor_id（找不到返回 nil）
func (d *DoctorDAO) FindIDByName(name string) *int64 {
	var doc model.DoctorWeWork
	err := d.db.Get(&doc, `SELECT * FROM doctor_wework WHERE doctor_name = ? AND is_active = 1`, name)
	if err != nil {
		return nil
	}
	return &doc.DoctorID
}

// List 分页查询医生映射
func (d *DoctorDAO) List(page, pageSize int) ([]model.DoctorWeWork, int, error) {
	var total int
	err := d.db.Get(&total, `SELECT COUNT(*) FROM doctor_wework`)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var docs []model.DoctorWeWork
	// go-mssqldb 对 OFFSET/FETCH 中的 ? 占位符支持不佳，整数直接内联（page/pageSize 已由 Atoi 校验，无注入风险）
	err = d.db.Select(&docs, fmt.Sprintf(`
		SELECT * FROM doctor_wework
		ORDER BY doctor_name ASC
		OFFSET %d ROWS FETCH NEXT %d ROWS ONLY
	`, offset, pageSize))
	if err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// Upsert 创建或更新医生映射
func (d *DoctorDAO) Upsert(doc *model.DoctorWeWork) error {
	_, err := d.db.Exec(`
		MERGE doctor_wework AS target
		USING (SELECT ? AS doctor_id) AS source
		ON target.doctor_id = source.doctor_id
		WHEN MATCHED THEN
			UPDATE SET doctor_name = ?, dept_id = ?, wework_userid = ?,
			           phone = ?, is_active = ?, updated_at = GETDATE()
		WHEN NOT MATCHED THEN
			INSERT (doctor_id, doctor_name, dept_id, wework_userid, phone, is_active)
			VALUES (?, ?, ?, ?, ?, ?)
	`, doc.DoctorID, doc.DoctorName, doc.DeptID, doc.WeWorkUserID,
		doc.Phone, doc.IsActive,
		doc.DoctorID, doc.DoctorName, doc.DeptID, doc.WeWorkUserID,
		doc.Phone, doc.IsActive)
	return err
}
