package session

// MemorySessionManager 使用内存实现的会话管理器
type MemorySessionManager struct {
	BaseSessionManager
}

// NewMemorySessionManager 创建一个新的内存会话管理器
func NewMemorySessionManager() *MemorySessionManager {
	return &MemorySessionManager{}
}
