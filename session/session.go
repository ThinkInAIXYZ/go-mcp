package session

import (
	"time"
)

// Manager 定义了会话管理的接口
type Manager interface {
	// CreateSession 创建新的会话，返回会话ID和状态
	CreateSession(data interface{}) (string, *State)

	// GetSession 获取会话状态
	GetSession(sessionID string) (*State, bool)

	// GetSessionChan 获取会话消息通道
	GetSessionChan(sessionID string) (chan []byte, bool)

	// UpdateSession 更新会话状态
	UpdateSession(sessionID string, updater func(*State) bool) bool

	// CloseSession 关闭并删除会话
	CloseSession(sessionID string)

	// CloseAllSessions 关闭所有会话
	CloseAllSessions()

	// CleanExpiredSessions 清理过期会话
	CleanExpiredSessions(maxIdleTime time.Duration)

	// RangeSessions 遍历所有会话
	RangeSessions(f func(sessionID string, state *State) bool)

	// SessionCount 获取会话数量
	SessionCount() int
}

// State 定义了会话的状态和数据
type State struct {
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

// Store 定义了之前的 session 存储接口 (为了向后兼容)
type Store interface {
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
	// CreateSession 创建新的会话，返回会话ID和消息通道
	CreateSession() (string, chan []byte)

	// GetSessionChan 获取会话消息通道
	GetSessionChan(sessionID string) (chan []byte, bool)

	// CloseSession 关闭会话
	CloseSession(sessionID string)
}
