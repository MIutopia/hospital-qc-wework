package model

import "time"

// Department 科室表
type Department struct {
	ID        int64     `db:"id" json:"id"`
	DeptName  string    `db:"dept_name" json:"deptName"`
	DeptCode  *string   `db:"dept_code" json:"deptCode"`
	ParentID  *int64    `db:"parent_id" json:"parentId"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}
