package push

import (
	"time"

	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/model"

	"github.com/rs/zerolog/log"
)

// RetryService 重试队列服务
type RetryService struct {
	pushLogDAO *dao.PushLogDAO
	pusher     *Pusher
}

// NewRetryService 创建重试服务
func NewRetryService(pushLogDAO *dao.PushLogDAO, pusher *Pusher) *RetryService {
	return &RetryService{
		pushLogDAO: pushLogDAO,
		pusher:     pusher,
	}
}

// RetryInterval 根据重试次数返回等待间隔
func RetryInterval(retryCount int) time.Duration {
	intervals := []int{1, 5, 15, 30} // 分钟
	if retryCount < 0 || retryCount >= len(intervals) {
		return 30 * time.Minute
	}
	return time.Duration(intervals[retryCount]) * time.Minute
}

// ProcessRetries 处理需要重试的推送（启动后台协程）
func (s *RetryService) ProcessRetries() {
	log.Info().Msg("重试队列处理开始")

	logs, err := s.pushLogDAO.GetPendingRetries(len(model.RetryIntervals))
	if err != nil {
		log.Warn().Err(err).Msg("查询待重试推送记录失败")
		return
	}

	if len(logs) == 0 {
		return
	}

	for _, l := range logs {
		// 检查重试间隔
		elapsed := time.Since(l.CreatedAt)
		expectedInterval := RetryInterval(l.RetryCount)

		if elapsed < expectedInterval {
			continue // 还没到重试时间
		}

		// TODO: 执行重试
		// 1. 根据 push_log 中的信息重建推送
		// 2. 调用 pusher.SendCard
		// 3. 更新推送状态
		_ = l
		log.Info().Int64("logId", l.ID).Int("retryCount", l.RetryCount).Msg("准备重试推送")
	}

	log.Info().Int("count", len(logs)).Msg("重试队列处理完成")
}
