package session

import (
	"go-mcp/pkg"
	"sync"
	"sync/atomic"
	"time"
)

// BaseSessionManager 提供会话管理器的基础实现
// 可被继承/嵌入到不同类型的会话管理器中
type BaseSessionManager struct {
	sessions pkg.SyncMap[string, *State] // 使用泛型SyncMap存储会话
	count    atomic.Int64                // 使用原子计数器跟踪会话数量
	mu       sync.Mutex                  // 保护并发访问
}

// CreateSession 创建新的会话
func (m *BaseSessionManager) CreateSession(data interface{}) (string, *State) {
	sessionID := pkg.GenerateUUID()
	now := time.Now()

	state := &State{
		ID:           sessionID,
		CreatedAt:    now,
		LastActiveAt: now,
		Data:         data,
		MessageChan:  make(chan []byte, 64),
	}

	m.sessions.Store(sessionID, state)
	m.count.Add(1) // 增加计数

	return sessionID, state
}

// GetSession 获取会话状态
func (m *BaseSessionManager) GetSession(sessionID string) (*State, bool) {
	state, has := m.sessions.Load(sessionID)
	if !has {
		return nil, false
	}

	// 更新最后活跃时间
	state.LastActiveAt = time.Now()

	return state, true
}

// GetSessionChan 获取会话消息通道
func (m *BaseSessionManager) GetSessionChan(sessionID string) (chan []byte, bool) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, false
	}

	return state.MessageChan, true
}

// UpdateSession 更新会话状态
func (m *BaseSessionManager) UpdateSession(sessionID string, updater func(*State) bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, has := m.sessions.Load(sessionID)
	if !has {
		return false
	}

	if updated := updater(state); updated {
		state.LastActiveAt = time.Now()
		return true
	}

	return false
}

// CloseSession 关闭并删除会话
func (m *BaseSessionManager) CloseSession(sessionID string) {
	state, ok := m.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}

	// 关闭消息通道
	close(state.MessageChan)
	m.count.Add(-1) // 减少计数
}

// CloseAllSessions 关闭所有会话
func (m *BaseSessionManager) CloseAllSessions() {
	m.sessions.Range(func(sessionID string, state *State) bool {
		// 关闭消息通道
		close(state.MessageChan)
		// 删除会话
		m.sessions.Delete(sessionID)
		return true
	})
	m.count.Store(0) // 重置计数为0
}

// CleanExpiredSessions 清理过期会话
func (m *BaseSessionManager) CleanExpiredSessions(maxIdleTime time.Duration) {
	now := time.Now()
	var expiredCount int64

	m.sessions.Range(func(sessionID string, state *State) bool {
		if now.Sub(state.LastActiveAt) > maxIdleTime {
			m.CloseSession(sessionID)
			expiredCount++
		}
		return true
	})
}

// RangeSessions 遍历所有会话
func (m *BaseSessionManager) RangeSessions(f func(sessionID string, state *State) bool) {
	m.sessions.Range(f)
}

// SessionCount 获取会话数量
func (m *BaseSessionManager) SessionCount() int {
	return int(m.count.Load())
}
