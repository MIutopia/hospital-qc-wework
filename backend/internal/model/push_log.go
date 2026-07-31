package model

import "time"

// PushLog 推送记录表
type PushLog struct {
	ID             int64      `db:"id" json:"id"`
	CaseID         int64      `db:"case_id" json:"caseId"`
	QCResultIDs    string     `db:"qc_result_ids" json:"qcResultIds"`       // JSON 数组, NVARCHAR(MAX)
	ReceiverUserID string     `db:"receiver_userid" json:"receiverUserid"`
	PushType       string     `db:"push_type" json:"pushType"`             // CARD / MARKDOWN / TEXT
	PushContent    *string    `db:"push_content" json:"pushContent"`       // JSON, NVARCHAR(MAX)
	PushStatus     string     `db:"push_status" json:"pushStatus"`         // PENDING / SUCCESS / FAILED / DEFERRED
	PushResponse   *string    `db:"push_response" json:"pushResponse"`
	RetryCount     int        `db:"retry_count" json:"retryCount"`
	PushedAt       *time.Time `db:"pushed_at" json:"pushedAt"`
	CreatedAt      time.Time  `db:"created_at" json:"createdAt"`
}

// 常量：push_type
const (
	PushTypeCard     = "CARD"
	PushTypeMarkdown = "MARKDOWN"
	PushTypeText     = "TEXT"
)

// 常量：push_status
const (
	PushStatusPending  = "PENDING"
	PushStatusSuccess  = "SUCCESS"
	PushStatusFailed   = "FAILED"
	PushStatusDeferred = "DEFERRED" // 免打扰延迟
)

// 重试策略（分钟）
var RetryIntervals = []int{1, 5, 15, 30}
