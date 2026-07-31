package dao

import (
	"hospital-qc-wework/internal/model"

	"github.com/jmoiron/sqlx"
)

// SyncLogDAO 数据同步日志数据访问
type SyncLogDAO struct {
	db *sqlx.DB
}

func NewSyncLogDAO(db *sqlx.DB) *SyncLogDAO {
	return &SyncLogDAO{db: db}
}

// Create 创建同步日志，返回新记录 ID（SQL Server 用 OUTPUT INSERTED.id 取回）
func (d *SyncLogDAO) Create(l *model.SyncLog) (int64, error) {
	var id int64
	err := d.db.Get(&id, `
		INSERT INTO sync_log (sync_type, status, started_at)
		OUTPUT INSERTED.id
		VALUES (?, ?, ?)
	`, l.SyncType, l.Status, l.StartedAt)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Update 更新同步日志终态（SUCCESS/FAILED）
func (d *SyncLogDAO) Update(id int64, l *model.SyncLog) error {
	_, err := d.db.Exec(`
		UPDATE sync_log
		SET status = ?, total_synced = ?, new_cases = ?, updated = ?,
		    error_msg = ?, finished_at = ?, elapsed_ms = ?
		WHERE id = ?
	`, l.Status, l.TotalSynced, l.NewCases, l.Updated,
		l.ErrorMsg, l.FinishedAt, l.ElapsedMS, id)
	return err
}

// List 分页查询同步日志（按开始时间倒序）
func (d *SyncLogDAO) List(page, pageSize int) ([]model.SyncLog, int, error) {
	var total int
	if err := d.db.Get(&total, `SELECT COUNT(*) FROM sync_log`); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var logs []model.SyncLog
	if err := d.db.Select(&logs, `
		SELECT * FROM sync_log
		ORDER BY started_at DESC
		OFFSET ? ROWS FETCH NEXT ? ROWS ONLY
	`, offset, pageSize); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
