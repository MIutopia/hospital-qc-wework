package tokenbucket

import (
	"sync"
	"time"
)

// Bucket 令牌桶，用于企业微信 API 限频
type Bucket struct {
	rate       float64   // 每秒放入令牌数
	capacity   float64   // 桶容量
	tokens     float64   // 当前令牌数
	lastRefill time.Time // 上次补充时间
	mu         sync.Mutex
}

// New 创建令牌桶
// rate: 每秒放入令牌数（例如 10 表示每秒 10 次）
// capacity: 桶容量（例如 100）
func New(rate, capacity float64) *Bucket {
	return &Bucket{
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// Allow 是否允许消费 n 个令牌
func (b *Bucket) Allow(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= float64(n) {
		b.tokens -= float64(n)
		return true
	}
	return false
}

// Wait 等待直到可以消费 n 个令牌（阻塞）
func (b *Bucket) Wait(n int) {
	for !b.Allow(n) {
		time.Sleep(50 * time.Millisecond)
	}
}

// WaitMax 等待直到可以消费 n 个令牌，但不超过 maxWait
// 返回 true=成功获取，false=超时
func (b *Bucket) WaitMax(n int, maxWait time.Duration) bool {
	deadline := time.Now().Add(maxWait)
	for !b.Allow(n) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
	return true
}

// refill 补充令牌
func (b *Bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastRefill = now
}
