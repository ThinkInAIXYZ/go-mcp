package pkg

import (
	"sync"
	"sync/atomic"
	"time"
)

// TimeWheelSessionManager 基于时间轮算法的会话管理器
// 相比MemorySessionManager，它可以更高效地处理会话过期
type TimeWheelSessionManager struct {
	sessions  sync.Map     // 存储所有会话
	count     atomic.Int64 // 会话计数
	timeWheel *TimeWheel   // 时间轮
	mu        sync.Mutex   // 互斥锁

	// 默认最大空闲时间
	defaultMaxIdleTime time.Duration
}

// NewTimeWheelSessionManager 创建一个新的基于时间轮的会话管理器
// 参数:
// - tickInterval: 时间轮的滴答间隔
// - wheelSize: 时间轮的槽位数量
// - defaultMaxIdleTime: 默认会话最大空闲时间
func NewTimeWheelSessionManager(tickInterval time.Duration, wheelSize int, defaultMaxIdleTime time.Duration) *TimeWheelSessionManager {
	m := &TimeWheelSessionManager{
		defaultMaxIdleTime: defaultMaxIdleTime,
	}

	// 创建时间轮，并设置过期任务的处理函数
	m.timeWheel = NewTimeWheel(tickInterval, wheelSize, func(task *Task) {
		m.handleExpiredSession(task.ID)
	})

	// 启动时间轮
	m.timeWheel.Start()

	return m
}

// Shutdown 关闭会话管理器
func (m *TimeWheelSessionManager) Shutdown() {
	// 停止时间轮
	if m.timeWheel != nil {
		m.timeWheel.Stop()
	}

	// 关闭所有会话
	m.CloseAllSessions()
}

// CreateSession 创建新的会话
func (m *TimeWheelSessionManager) CreateSession(data interface{}) (string, *SessionState) {
	sessionID := GenerateUUID()
	now := time.Now()

	state := &SessionState{
		ID:           sessionID,
		CreatedAt:    now,
		LastActiveAt: now,
		Data:         data,
		MessageChan:  make(chan []byte, 64),
	}

	m.sessions.Store(sessionID, state)
	m.count.Add(1) // 增加计数

	// 添加到时间轮，设置过期时间
	if m.timeWheel != nil {
		m.timeWheel.AddTask(sessionID, m.defaultMaxIdleTime, nil)
	}

	return sessionID, state
}

// GetSession 获取会话状态并刷新最后活跃时间
func (m *TimeWheelSessionManager) GetSession(sessionID string) (*SessionState, bool) {
	value, has := m.sessions.Load(sessionID)
	if !has {
		return nil, false
	}

	state, ok := value.(*SessionState)
	if !ok {
		return nil, false
	}

	// 更新最后活跃时间
	now := time.Now()
	oldTime := state.LastActiveAt
	state.LastActiveAt = now

	// 更新时间轮任务，使用新的过期时间
	// 只有当会话已经活跃较长时间，才更新时间轮任务，避免频繁更新
	if now.Sub(oldTime) > m.defaultMaxIdleTime/4 && m.timeWheel != nil {
		m.timeWheel.AddTask(sessionID, m.defaultMaxIdleTime, nil)
	}

	return state, true
}

// GetSessionChan 获取会话消息通道
func (m *TimeWheelSessionManager) GetSessionChan(sessionID string) (chan []byte, bool) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, false
	}

	return state.MessageChan, true
}

// UpdateSession 更新会话状态
// 此方法优化了时间轮任务的更新逻辑，提高效率
func (m *TimeWheelSessionManager) UpdateSession(sessionID string, updater func(*SessionState) bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	value, has := m.sessions.Load(sessionID)
	if !has {
		return false
	}

	state, ok := value.(*SessionState)
	if !ok {
		return false
	}

	oldTime := state.LastActiveAt
	now := time.Now()

	// 调用更新函数
	if updated := updater(state); !updated {
		return false
	}

	// 确保LastActiveAt已被更新为当前时间
	state.LastActiveAt = now

	// 更新时间轮任务，使用新的过期时间
	// 只有当会话已经活跃较长时间，才更新时间轮任务，避免频繁更新
	if now.Sub(oldTime) > m.defaultMaxIdleTime/4 && m.timeWheel != nil {
		m.timeWheel.AddTask(sessionID, m.defaultMaxIdleTime, nil)
	}

	return true
}

// CloseSession 关闭并删除会话
func (m *TimeWheelSessionManager) CloseSession(sessionID string) {
	// 从时间轮中移除任务
	if m.timeWheel != nil {
		m.timeWheel.RemoveTask(sessionID)
	}

	value, ok := m.sessions.Load(sessionID)
	if !ok {
		return
	}

	state, ok := value.(*SessionState)
	if !ok {
		return
	}

	// 关闭消息通道
	close(state.MessageChan)
	m.sessions.Delete(sessionID)
	m.count.Add(-1) // 减少计数
}

// CloseAllSessions 关闭所有会话
func (m *TimeWheelSessionManager) CloseAllSessions() {
	m.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		m.CloseSession(sessionID)
		return true
	})
	m.count.Store(0) // 重置计数为0
}

// CleanExpiredSessions 清理过期会话
// 时间轮会话管理器已经通过时间轮自动处理过期，
// 这个方法主要用于兼容接口和手动触发清理
func (m *TimeWheelSessionManager) CleanExpiredSessions(maxIdleTime time.Duration) {
	// 使用时间轮进行会话过期管理，所以这里只需将未添加的会话加入时间轮
	now := time.Now()

	m.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		state, ok := value.(*SessionState)
		if !ok {
			return true
		}

		// 计算剩余的空闲时间
		idleTime := now.Sub(state.LastActiveAt)
		if idleTime > maxIdleTime {
			// 会话已过期，立即关闭
			m.CloseSession(sessionID)
		} else if m.timeWheel != nil {
			// 将会话添加到时间轮中，设置剩余的空闲时间作为过期时间
			remainingTime := maxIdleTime - idleTime
			m.timeWheel.AddTask(sessionID, remainingTime, nil)
		}

		return true
	})
}

// handleExpiredSession 处理过期的会话
// 此方法会被时间轮调用
func (m *TimeWheelSessionManager) handleExpiredSession(sessionID string) {
	// 检查会话是否真的过期
	value, has := m.sessions.Load(sessionID)
	if !has {
		return
	}

	state, ok := value.(*SessionState)
	if !ok {
		return
	}

	// 再次检查会话是否真的过期，以防在时间轮触发和处理之间会话被更新
	if time.Since(state.LastActiveAt) < m.defaultMaxIdleTime {
		// 会话没有过期，重新添加到时间轮
		if m.timeWheel != nil {
			remainingTime := m.defaultMaxIdleTime - time.Since(state.LastActiveAt)
			m.timeWheel.AddTask(sessionID, remainingTime, nil)
		}
		return
	}

	// 会话确实过期，关闭它
	m.CloseSession(sessionID)
}

// RangeSessions 遍历所有会话
func (m *TimeWheelSessionManager) RangeSessions(f func(sessionID string, state *SessionState) bool) {
	m.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		state, ok := value.(*SessionState)
		if !ok {
			return true
		}

		return f(sessionID, state)
	})
}

// SessionCount 获取会话数量
func (m *TimeWheelSessionManager) SessionCount() int {
	return int(m.count.Load())
}
