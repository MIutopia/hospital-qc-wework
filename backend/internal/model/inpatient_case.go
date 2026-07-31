package model

import "time"

// InpatientCase 住院病例表
type InpatientCase struct {
	ID            int64      `db:"id" json:"id"`
	CaseNo        string     `db:"case_no" json:"caseNo"`
	PatientName   string     `db:"patient_name" json:"patientName"`
	PatientGender *int       `db:"patient_gender" json:"patientGender"`
	PatientAge    *int       `db:"patient_age" json:"patientAge"`
	AdmitTime     time.Time  `db:"admit_time" json:"admitTime"`
	DischargeTime *time.Time `db:"discharge_time" json:"dischargeTime"`
	DeptID        int64      `db:"dept_id" json:"deptId"`
	DeptName      *string    `db:"dept_name" json:"deptName"`
	DoctorID      *int64     `db:"doctor_id" json:"doctorId"`
	DoctorName    *string    `db:"doctor_name" json:"doctorName"`
	Diagnosis     *string    `db:"diagnosis" json:"diagnosis"`
	CaseStatus    string     `db:"case_status" json:"caseStatus"`
	RawData       *string    `db:"raw_data" json:"rawData"`           // JSON, NVARCHAR(MAX)
	SyncTime      *time.Time `db:"sync_time" json:"syncTime"`
	QCStatus      string     `db:"qc_status" json:"qcStatus"`         // PENDING / PASSED / ISSUED
	QCTime        *time.Time `db:"qc_time" json:"qcTime"`
	CreatedAt     time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updatedAt"`
}

// 常量：qc_status
const (
	QCStatusPending = "PENDING"
	QCStatusPassed  = "PASSED"
	QCStatusIssued  = "ISSUED"
)

// 常量：case_status
const (
	CaseStatusActive   = "ACTIVE"
	CaseStatusArchived = "ARCHIVED"
)
