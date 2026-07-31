package sync

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// SyncService 数据同步服务
type SyncService struct {
	// TODO: M2 阶段注入 DAO 和 HIS 数据库连接
}

// NewSyncService 创建同步服务
func NewSyncService() *SyncService {
	return &SyncService{}
}

// SyncResult 同步结果
type SyncResult struct {
	TotalSynced int    `json:"totalSynced"`
	NewCases    int    `json:"newCases"`
	Elapsed     string `json:"elapsed"`
}

// RunSync 执行数据同步
func (s *SyncService) RunSync() (*SyncResult, error) {
	start := time.Now()
	log.Info().Msg("数据同步开始")

	// TODO: M2 阶段实现
	// 1. 连接 HIS 只读库
	// 2. 查询增量的病例数据
	// 3. 写入 inpatient_case 表
	// 4. 记录同步日志

	elapsed := time.Since(start)
	log.Info().Str("elapsed", elapsed.Round(time.Millisecond).String()).Msg("数据同步完成（未实现）")

	return &SyncResult{
		TotalSynced: 0,
		NewCases:    0,
		Elapsed:     elapsed.Round(time.Millisecond).String(),
	}, nil
}

// SyncFromCSV 从 CSV 文件导入数据（阶段一兜底方案）
func (s *SyncService) SyncFromCSV(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("文件路径为空")
	}

	// TODO: M2 阶段实现 CSV 导入
	// 1. 读取 CSV 文件
	// 2. 按行解析并写入 inpatient_case
	// 3. 同步医生映射

	log.Info().Str("path", filePath).Msg("CSV 导入功能未实现")
	return nil
}
