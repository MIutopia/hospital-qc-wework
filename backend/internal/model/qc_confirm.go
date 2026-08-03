package model

import "time"

// QCConfirm 确认整改记录表
type QCConfirm struct {
	ID            int64      `db:"id" json:"id"`
	CaseID        int64      `db:"case_id" json:"caseId"`
	DoctorID      int64      `db:"doctor_id" json:"doctorId"`
	DefectIDs     *string    `db:"defect_ids" json:"defectIds"`     // JSON 数组，确认的缺陷ID列表
	ConfirmStatus string     `db:"confirm_status" json:"confirmStatus"` // PENDING / CONFIRMED / REJECTED
	ConfirmNote   *string    `db:"confirm_note" json:"confirmNote"`
	ConfirmedAt   *time.Time `db:"confirmed_at" json:"confirmedAt"`
	CreatedAt     time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updatedAt"`
}

// 常量：confirm_status
const (
	ConfirmStatusPending   = "PENDING"
	ConfirmStatusConfirmed = "CONFIRMED"
	ConfirmStatusRejected  = "REJECTED"
)
