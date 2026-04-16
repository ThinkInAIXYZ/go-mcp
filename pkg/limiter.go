package pkg

import (
	"sync"
	"time"
)

// RateLimiter 定义速率限制接口
type RateLimiter interface {
	Allow(toolName string) bool
}

// TokenBucketLimiter 令牌桶限速器实现
type TokenBucketLimiter struct {
	mu           sync.RWMutex
	buckets      map[string]*bucket
	defaultLimit Rate
	toolLimits   map[string]Rate
}

// Rate 定义速率限制参数
type Rate struct {
	Limit float64 // 每秒允许的请求数
	Burst int     // 突发请求上限
}

// bucket 令牌桶
type bucket struct {
	tokens        float64
	lastTimestamp time.Time
	rate          Rate
}

// NewTokenBucketLimiter 创建新的令牌桶限速器
func NewTokenBucketLimiter(defaultRate Rate) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		buckets:      make(map[string]*bucket),
		defaultLimit: defaultRate,
		toolLimits:   make(map[string]Rate),
	}
}

// SetToolLimit 为特定工具设置限制
func (l *TokenBucketLimiter) SetToolLimit(toolName string, rate Rate) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.toolLimits[toolName] = rate
	// 如果已有桶，更新其速率
	if b, exists := l.buckets[toolName]; exists {
		b.rate = rate
	}
}

// Allow 检查请求是否被允许
func (l *TokenBucketLimiter) Allow(toolName string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()

	// 获取或创建桶
	b, exists := l.buckets[toolName]
	if !exists {
		// 查找工具特定的限制，如果没有则使用默认限制
		rate, exists := l.toolLimits[toolName]
		if !exists {
			rate = l.defaultLimit
		}

		b = &bucket{
			tokens:        float64(rate.Burst),
			lastTimestamp: now,
			rate:          rate,
		}
		l.buckets[toolName] = b
	}

	// 计算从上次请求到现在应该添加的令牌
	elapsed := now.Sub(b.lastTimestamp).Seconds()
	b.lastTimestamp = now

	// 添加令牌，但不超过最大值
	b.tokens += elapsed * b.rate.Limit
	if b.tokens > float64(b.rate.Burst) {
		b.tokens = float64(b.rate.Burst)
	}

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}
