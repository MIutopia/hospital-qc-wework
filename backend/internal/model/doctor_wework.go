package model

import "time"

// DoctorWeWork 医生-企业微信映射表
type DoctorWeWork struct {
	ID           int64     `db:"id" json:"id"`
	DoctorID     int64     `db:"doctor_id" json:"doctorId"`
	DoctorName   string    `db:"doctor_name" json:"doctorName"`
	DeptID       *int64    `db:"dept_id" json:"deptId"`
	WeWorkUserID string    `db:"wework_userid" json:"weworkUserid"`
	Phone        *string   `db:"phone" json:"phone"`
	IsActive     int       `db:"is_active" json:"isActive"` // 0/1
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}
