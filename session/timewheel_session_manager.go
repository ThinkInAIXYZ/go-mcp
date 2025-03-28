package session

import (
	"go-mcp/pkg"
	"time"
)

// TimeWheelSessionManager 基于时间轮算法的会话管理器
// 相比MemorySessionManager，它可以更高效地处理会话过期
type TimeWheelSessionManager struct {
	BaseSessionManager
	timeWheel          *pkg.TimeWheel // 时间轮
	defaultMaxIdleTime time.Duration  // 默认会话最大空闲时间
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
	m.timeWheel = pkg.NewTimeWheel(tickInterval, wheelSize, func(task *pkg.Task) {
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

// CreateSession 重写CreateSession方法，添加时间轮任务
func (m *TimeWheelSessionManager) CreateSession(data interface{}) (string, *State) {
	// 调用基础实现创建会话
	sessionID, state := m.BaseSessionManager.CreateSession(data)

	// 添加到时间轮，设置过期时间
	if m.timeWheel != nil {
		m.timeWheel.AddTask(sessionID, m.defaultMaxIdleTime, nil)
	}

	return sessionID, state
}

// GetSession 重写GetSession方法，更新时间轮任务
func (m *TimeWheelSessionManager) GetSession(sessionID string) (*State, bool) {
	state, has := m.BaseSessionManager.GetSession(sessionID)
	if !has {
		return nil, false
	}

	// 更新时间轮任务，但避免频繁更新
	// 只有当会话已经活跃较长时间，才更新时间轮任务
	now := time.Now()
	if now.Sub(state.LastActiveAt) > m.defaultMaxIdleTime/4 && m.timeWheel != nil {
		m.timeWheel.AddTask(sessionID, m.defaultMaxIdleTime, nil)
	}

	return state, true
}

// UpdateSession 重写UpdateSession方法，更新时间轮任务
func (m *TimeWheelSessionManager) UpdateSession(sessionID string, updater func(*State) bool) bool {
	// 获取修改前的时间
	var oldTime time.Time
	state, has := m.sessions.Load(sessionID)
	if has {
		oldTime = state.LastActiveAt
	}

	// 调用基础实现更新会话
	if !m.BaseSessionManager.UpdateSession(sessionID, updater) {
		return false
	}

	// 更新时间轮任务，但避免频繁更新
	now := time.Now()
	if now.Sub(oldTime) > m.defaultMaxIdleTime/4 && m.timeWheel != nil {
		m.timeWheel.AddTask(sessionID, m.defaultMaxIdleTime, nil)
	}

	return true
}

// CloseSession 重写CloseSession方法，移除时间轮任务
func (m *TimeWheelSessionManager) CloseSession(sessionID string) {
	// 从时间轮中移除任务
	if m.timeWheel != nil {
		m.timeWheel.RemoveTask(sessionID)
	}

	// 调用基础实现关闭会话
	m.BaseSessionManager.CloseSession(sessionID)
}

// CleanExpiredSessions 重写CleanExpiredSessions方法，利用时间轮管理过期
func (m *TimeWheelSessionManager) CleanExpiredSessions(maxIdleTime time.Duration) {
	// 由于已经使用时间轮管理过期，这里只处理那些可能没有添加到时间轮的会话
	now := time.Now()

	m.sessions.Range(func(sessionID string, state *State) bool {
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
	state, has := m.sessions.Load(sessionID)
	if !has {
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
