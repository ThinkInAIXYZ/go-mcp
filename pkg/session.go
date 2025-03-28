package pkg

import (
	"sync"
	"sync/atomic"
	"time"
)

// SessionManager 定义了会话管理的接口
type SessionManager interface {
	// CreateSession 创建新的会话，返回会话ID和状态
	CreateSession(data interface{}) (string, *SessionState)

	// GetSession 获取会话状态
	GetSession(sessionID string) (*SessionState, bool)

	// GetSessionChan 获取会话消息通道
	GetSessionChan(sessionID string) (chan []byte, bool)

	// UpdateSession 更新会话状态
	UpdateSession(sessionID string, updater func(*SessionState) bool) bool

	// CloseSession 关闭并删除会话
	CloseSession(sessionID string)

	// CloseAllSessions 关闭所有会话
	CloseAllSessions()

	// CleanExpiredSessions 清理过期会话
	CleanExpiredSessions(maxIdleTime time.Duration)

	// RangeSessions 遍历所有会话
	RangeSessions(f func(sessionID string, state *SessionState) bool)

	// SessionCount 获取会话数量
	SessionCount() int
}

// SessionState 定义了会话的状态和数据
type SessionState struct {
	// 会话ID
	ID string

	// 会话创建时间
	CreatedAt time.Time

	// 会话最后活跃时间
	LastActiveAt time.Time

	// 存储会话的自定义数据，由使用者自行管理
	Data interface{}

	// 会话的消息通道，用于发送消息到客户端
	MessageChan chan []byte
}

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
func (m *MemorySessionManager) CreateSession(data interface{}) (string, *SessionState) {
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
	return sessionID, state
}

// GetSession 获取会话状态
func (m *MemorySessionManager) GetSession(sessionID string) (*SessionState, bool) {
	value, has := m.sessions.Load(sessionID)
	if !has {
		return nil, false
	}

	state, ok := value.(*SessionState)
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
func (m *MemorySessionManager) UpdateSession(sessionID string, updater func(*SessionState) bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	value, has := m.sessions.Load(sessionID)
	if !has {
		return false
	}

	state, ok := value.(*SessionState)
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
		state, ok := value.(*SessionState)
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
func (m *MemorySessionManager) RangeSessions(f func(sessionID string, state *SessionState) bool) {
	m.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		state, ok := value.(*SessionState)
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

// SessionStore 定义了之前的 session 存储接口 (为了向后兼容)
type SessionStore interface {
	// Store 存储一个 session
	Store(key string, value interface{})
	// Load 加载一个 session
	Load(key string) (interface{}, bool)
	// Delete 删除一个 session
	Delete(key string)
	// Range 遍历所有 session
	Range(f func(key string, value interface{}) bool)
}

// TransportSessionManager 是专门为transport层设计的简化版会话管理接口
type TransportSessionManager interface {
	// 创建新的会话，返回会话ID和消息通道
	CreateSession() (string, chan []byte)

	// 获取会话消息通道
	GetSessionChan(sessionID string) (chan []byte, bool)

	// 关闭会话
	CloseSession(sessionID string)
}
