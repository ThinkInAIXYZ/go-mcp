package session

import (
	"go-mcp/pkg"
	"sync"
	"sync/atomic"
	"time"
)

// MemorySessionManager 使用内存实现的会话管理器
type MemorySessionManager struct {
	sessions sync.Map
	count    atomic.Int64 // 用于跟踪会话数量
	mutex    sync.Mutex
}

// NewMemorySessionManager 创建一个新的内存会话管理器
func NewMemorySessionManager() *MemorySessionManager {
	return &MemorySessionManager{}
}

// CreateSession 创建新的会话
func (m *MemorySessionManager) CreateSession(data interface{}) (string, *State) {
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
func (m *MemorySessionManager) GetSession(sessionID string) (*State, bool) {
	value, has := m.sessions.Load(sessionID)
	if !has {
		return nil, false
	}

	state, ok := value.(*State)
	if !ok {
		return nil, false
	}

	// 更新最后活跃时间
	state.LastActiveAt = time.Now()
	return state, true
}

// GetSessionChan 获取会话消息通道
func (m *MemorySessionManager) GetSessionChan(sessionID string) (chan []byte, bool) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, false
	}

	return state.MessageChan, true
}

// UpdateSession 更新会话状态
func (m *MemorySessionManager) UpdateSession(sessionID string, updater func(*State) bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	value, has := m.sessions.Load(sessionID)
	if !has {
		return false
	}

	state, ok := value.(*State)
	if !ok {
		return false
	}

	if updated := updater(state); updated {
		state.LastActiveAt = time.Now()
		return true
	}

	return false
}

// CloseSession 关闭并删除会话
func (m *MemorySessionManager) CloseSession(sessionID string) {
	value, ok := m.sessions.Load(sessionID)
	if !ok {
		return
	}

	state, ok := value.(*State)
	if !ok {
		return
	}

	// 关闭消息通道
	close(state.MessageChan)
	m.sessions.Delete(sessionID)
	m.count.Add(-1) // 减少计数
}

// CloseAllSessions 关闭所有会话
func (m *MemorySessionManager) CloseAllSessions() {
	m.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		m.CloseSession(sessionID)
		return true
	})
	m.count.Store(0) // 重置计数为0
}

// CleanExpiredSessions 清理过期会话
func (m *MemorySessionManager) CleanExpiredSessions(maxIdleTime time.Duration) {
	now := time.Now()
	expiredCount := int64(0)

	m.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		state, ok := value.(*State)
		if !ok {
			return true
		}

		if now.Sub(state.LastActiveAt) > maxIdleTime {
			m.sessions.Delete(sessionID)
			close(state.MessageChan)
			expiredCount++
		}
		return true
	})

	if expiredCount > 0 {
		m.count.Add(-expiredCount) // 更新计数
	}
}

// RangeSessions 遍历所有会话
func (m *MemorySessionManager) RangeSessions(f func(sessionID string, state *State) bool) {
	m.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		state, ok := value.(*State)
		if !ok {
			return true
		}

		return f(sessionID, state)
	})
}

// SessionCount 获取会话数量，O(1)复杂度
func (m *MemorySessionManager) SessionCount() int {
	return int(m.count.Load())
}
