package model

import "time"

// SyncLog 数据同步日志（HIS 增量同步 / CSV 导入）
type SyncLog struct {
	ID          int64      `db:"id" json:"id"`
	SyncType    string     `db:"sync_type" json:"syncType"`    // HIS / CSV
	Status      string     `db:"status" json:"status"`         // RUNNING / SUCCESS / FAILED
	TotalSynced int        `db:"total_synced" json:"totalSynced"`
	NewCases    int        `db:"new_cases" json:"newCases"`
	Updated     int        `db:"updated" json:"updated"`
	ErrorMsg    *string    `db:"error_msg" json:"errorMsg"`
	StartedAt   time.Time  `db:"started_at" json:"startedAt"`
	FinishedAt  *time.Time `db:"finished_at" json:"finishedAt"`
	ElapsedMS   *int64     `db:"elapsed_ms" json:"elapsedMs"`
}

// 同步类型常量
const (
	SyncTypeHIS = "HIS"
	SyncTypeCSV = "CSV"
)

// 同步状态常量
const (
	SyncStatusRunning = "RUNNING"
	SyncStatusSuccess = "SUCCESS"
	SyncStatusFailed  = "FAILED"
)
