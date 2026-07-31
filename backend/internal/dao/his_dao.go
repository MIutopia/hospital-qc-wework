package dao

import (
	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// HISDAO 访问 HIS 数据仓库只读数据（库名由配置 his_database.name 指定）
type HISDAO struct {
	db *sqlx.DB
}

func NewHISDAO(db *sqlx.DB) *HISDAO {
	return &HISDAO{db: db}
}

// QueryNewCases 增量查询病案首页数据（录入时间 > since）
// HIS 数据仓库各表均有「录入时间」字段，作为增量同步标识。
func (d *HISDAO) QueryNewCases(since *string, limit int) ([]model.HISCaseman, error) {
	var cases []model.HISCaseman

	query := `
		SELECT TOP (?)
		       首页序列, 就诊卡号, 住院号, 姓名, 性别, 住院年龄, 出生日期,
		       入院时间, 出院时间, 住院天数, 出院标识,
		       当前科室, 当前病区, 入院科室, 入院病区, 出院科室, 出院方式,
		       门急诊中医诊断, 门急诊西医诊断, 西医诊断_疾病编码, 中医诊断_疾病编码,
		       病理诊断内容, 手术编码, 手术名称, 术者, 麻醉方式,
		       主任医师, 主治医师, 责任护士, 住院医师, 病历质量,
		       质控日期, 质控医生, 录入时间
		FROM dbo.hospitalisation_case_man
	`

	// 增量模式：只取录入时间晚于同步断点的数据
	args := []interface{}{limit}
	if since != nil && *since != "" {
		query += ` WHERE 录入时间 > ? ORDER BY 录入时间 ASC`
		args = append(args, *since)
	} else {
		// 首次全量：取最近 30 天数据，避免一次性拉取历史全量
		query += ` WHERE 录入时间 >= DATEADD(DAY, -30, GETDATE()) ORDER BY 录入时间 ASC`
	}

	if err := d.db.Select(&cases, query, args...); err != nil {
		return nil, err
	}
	return cases, nil
}

// QueryAdmissionRecords 查询入院记录（补充主诉/现病史等病历内容）
// HIS 入院记录表无住院号列，用「姓名 + 入院时间」匹配关联。
func (d *HISDAO) QueryAdmissionRecords(since *string, limit int) ([]model.HISAdmissionRecord, error) {
	var records []model.HISAdmissionRecord

	query := `
		SELECT TOP (?)
		       患者姓名, 性别, 年龄, 入院时间, 记录时间,
		       主诉, 现病史, 既往史, 个人史, 婚育史, 月经史, 家族史,
		       体格检查, 专科情况, 辅助检查,
		       中医初步诊断, 西医初步诊断, 录入时间
		FROM dbo.med_record_hospitail_rceord
	`

	args := []interface{}{limit}
	if since != nil && *since != "" {
		query += ` WHERE 录入时间 > ? ORDER BY 录入时间 ASC`
		args = append(args, *since)
	} else {
		query += ` WHERE 录入时间 >= DATEADD(DAY, -30, GETDATE()) ORDER BY 录入时间 ASC`
	}

	if err := d.db.Select(&records, query, args...); err != nil {
		return nil, err
	}
	return records, nil
}
