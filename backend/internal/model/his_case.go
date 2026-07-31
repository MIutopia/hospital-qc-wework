package model

import "time"

// HISCaseman 对应 HIS 数据仓库 med_record 库的 hospitalisation_case_man 表（住院病案首页宽表）
// 仅映射质控系统需要的字段；HIS 表为中文列名，通过 db 标签映射。
type HISCaseman struct {
	CaseSerial        int        `db:"首页序列"`
	ICNo              string     `db:"就诊卡号"`
	HospitalNo        string     `db:"住院号"`
	Name              string     `db:"姓名"`
	Gender            string     `db:"性别"`
	Age               int        `db:"住院年龄"`
	BirthDate         *time.Time `db:"出生日期"`
	AdmitTime         *time.Time `db:"入院时间"`
	DischargeTime     *time.Time `db:"出院时间"`
	InhospitalDays    *int       `db:"住院天数"`
	DischargeMark     string     `db:"出院标识"`
	CurrentDept       *string    `db:"当前科室"`
	CurrentWard       *string    `db:"当前病区"`
	AdmitDept         *string    `db:"入院科室"`
	AdmitWard         *string    `db:"入院病区"`
	DischargeDept     *string    `db:"出院科室"`
	DischargeMode     string     `db:"出院方式"`
	OutpatientTCMDiag *string    `db:"门急诊中医诊断"`
	OutpatientWDiag   *string    `db:"门急诊西医诊断"`
	WesternICD        *string    `db:"西医诊断_疾病编码"`
	TCMICD            *string    `db:"中医诊断_疾病编码"`
	PathologyDiag     *string    `db:"病理诊断内容"`
	SurgeryCode       *string    `db:"手术编码"`
	SurgeryName       *string    `db:"手术名称"`
	Surgeon           *string    `db:"术者"`
	Anesthesia        *string    `db:"麻醉方式"`
	ChiefDoctor       *string    `db:"主任医师"`
	AttendingDoctor   *string    `db:"主治医师"`
	ChargeNurse       *string    `db:"责任护士"`
	ResidentDoctor    *string    `db:"住院医师"`
	CaseQuality       *string    `db:"病历质量"`
	QCDate            *time.Time `db:"质控日期"`
	QCDoctor          *string    `db:"质控医生"`
	InputTime         *time.Time `db:"录入时间"`
}

// HISAdmissionRecord 对应 med_record 库的 med_record_hospitail_rceord 表（住院入院记录）
// 用于补充主诉、现病史等病历文书内容（存入 raw_data JSON）。
type HISAdmissionRecord struct {
	PatientName       string     `db:"患者姓名"`
	Gender            string     `db:"性别"`
	Age               *int       `db:"年龄"`
	AdmitTime         *time.Time `db:"入院时间"`
	RecordTime        *time.Time `db:"记录时间"`
	ChiefComplaint    *string    `db:"主诉"`
	PresentHistory    *string    `db:"现病史"`
	PastHistory       *string    `db:"既往史"`
	PersonalHistory   *string    `db:"个人史"`
	MaritalHistory    *string    `db:"婚育史"`
	MenstrualHistory  *string    `db:"月经史"`
	FamilyHistory     *string    `db:"家族史"`
	PhysicalExam      *string    `db:"体格检查"`
	SpecialtyExam     *string    `db:"专科情况"`
	AuxiliaryExam     *string    `db:"辅助检查"`
	TCMInitialDiag    *string    `db:"中医初步诊断"`
	WesternInitDiag   *string    `db:"西医初步诊断"`
	InputTime         *time.Time `db:"录入时间"`
}
