package dao

import (
	"fmt"

	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// PushLogDAO 推送日志数据访问
type PushLogDAO struct {
	db *sqlx.DB
}

func NewPushLogDAO(db *sqlx.DB) *PushLogDAO {
	return &PushLogDAO{db: db}
}

// Create 创建推送记录
func (d *PushLogDAO) Create(log *model.PushLog) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO push_log (case_id, qc_result_ids, receiver_userid,
		                      push_type, push_content, push_status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, log.CaseID, log.QCResultIDs, log.ReceiverUserID,
		log.PushType, log.PushContent, log.PushStatus)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateStatus 更新推送状态
func (d *PushLogDAO) UpdateStatus(id int64, status string, response string) error {
	_, err := d.db.Exec(`
		UPDATE push_log
		SET push_status = ?, push_response = ?,
		    pushed_at = CASE WHEN ? = 'SUCCESS' THEN GETDATE() ELSE pushed_at END,
		    retry_count = retry_count + 1
		WHERE id = ?
	`, status, response, status, id)
	return err
}

// GetPendingRetries 获取需要重试的推送记录
func (d *PushLogDAO) GetPendingRetries(maxRetries int) ([]model.PushLog, error) {
	var logs []model.PushLog
	err := d.db.Select(&logs, `
		SELECT id, case_id, qc_result_ids, receiver_userid,
		       push_type, push_content, push_status, retry_count
		FROM push_log
		WHERE push_status IN ('PENDING', 'FAILED')
		  AND retry_count < ?
		ORDER BY created_at ASC
	`, maxRetries)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByCaseID 根据病例ID查询推送记录
func (d *PushLogDAO) GetByCaseID(caseID int64) ([]model.PushLog, error) {
	var logs []model.PushLog
	err := d.db.Select(&logs, `
		SELECT * FROM push_log WHERE case_id = ? ORDER BY created_at DESC
	`, caseID)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// List 分页查询推送日志
func (d *PushLogDAO) List(page, pageSize int, status string) ([]model.PushLog, int, error) {
	var total int
	query := `SELECT COUNT(*) FROM push_log`
	args := []interface{}{}
	if status != "" {
		query += ` WHERE push_status = ?`
		args = append(args, status)
	}
	err := d.db.Get(&total, query, args...)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var logs []model.PushLog
	dataQuery := `SELECT * FROM push_log`
	if status != "" {
		dataQuery += ` WHERE push_status = ?`
	}
	// go-mssqldb 对 OFFSET/FETCH 中的 ? 占位符支持不佳，整数直接内联（page/pageSize 已由 Atoi 校验，无注入风险）
	dataQuery += fmt.Sprintf(` ORDER BY created_at DESC OFFSET %d ROWS FETCH NEXT %d ROWS ONLY`, offset, pageSize)
	err = d.db.Select(&logs, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
