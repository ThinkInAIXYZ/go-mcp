package pkg

import "time"

// SessionManagerType 定义会话管理器的类型
type SessionManagerType string

const (
	// MemorySessionManagerType 内存会话管理器类型
	MemorySessionManagerType SessionManagerType = "memory"
	
	// TimeWheelSessionManagerType 时间轮会话管理器类型
	TimeWheelSessionManagerType SessionManagerType = "timewheel"
)

// SessionManagerOptions 会话管理器的配置选项
type SessionManagerOptions struct {
	// 时间轮配置
	TickInterval     time.Duration
	WheelSize        int
	DefaultMaxIdleTime time.Duration
}

// DefaultSessionManagerOptions 返回默认的会话管理器选项
func DefaultSessionManagerOptions() *SessionManagerOptions {
	return &SessionManagerOptions{
		TickInterval:     time.Second,      // 默认1秒一个刻度
		WheelSize:        60,               // 默认60个槽位
		DefaultMaxIdleTime: 30 * time.Minute, // 默认30分钟过期
	}
}

// NewSessionManager 创建一个新的会话管理器
// 工厂方法，根据类型创建不同的会话管理器实现
func NewSessionManager(managerType SessionManagerType, options *SessionManagerOptions) SessionManager {
	if options == nil {
		options = DefaultSessionManagerOptions()
	}
	
	switch managerType {
	case TimeWheelSessionManagerType:
		return NewTimeWheelSessionManager(options.TickInterval, options.WheelSize, options.DefaultMaxIdleTime)
	case MemorySessionManagerType:
		fallthrough
	default:
		return NewMemorySessionManager()
	}
} 