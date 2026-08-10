package core

import (
	"sync"
	"time"
)

// sampler 按秒限流：同一秒内最多放行 max 条日志，超出部分丢弃。
// 用于故障风暴场景下保护磁盘 IO 与服务稳定性。
type sampler struct {
	max int
	now func() time.Time

	mu          sync.Mutex
	windowStart time.Time
	count       int
}

// newSampler 创建一个按秒限流采样器。
func newSampler(maxPerSecond int) *sampler {
	return &sampler{max: maxPerSecond, now: time.Now}
}

// allow 判断当前日志是否允许输出。
func (s *sampler) allow() bool {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.Sub(s.windowStart) >= time.Second {
		s.windowStart = now
		s.count = 0
	}
	if s.count >= s.max {
		return false
	}
	s.count++
	return true
}
