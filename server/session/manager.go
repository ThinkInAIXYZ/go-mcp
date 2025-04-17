package session

import (
	"context"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type Manager struct {
	sessions pkg.SyncMap[*State]

	stopHeartbeat chan struct{}

	detection   func(ctx context.Context, sessionID string) error
	maxIdleTime time.Duration
}

func NewManager(detection func(ctx context.Context, sessionID string) error) *Manager {
	return &Manager{
		detection:     detection,
		stopHeartbeat: make(chan struct{}),
	}
}

func (m *Manager) SetMaxIdleTime(d time.Duration) {
	m.maxIdleTime = d
}

func (m *Manager) CreateSession(sessionID string) {
	state := NewState()
	m.sessions.Store(sessionID, state)
}

func (m *Manager) IsExistSession(sessionID string) bool {
	_, has := m.sessions.Load(sessionID)
	return has
}

func (m *Manager) GetSession(sessionID string) (*State, bool) {
	state, has := m.sessions.Load(sessionID)
	if !has {
		return nil, false
	}
	return state, true
}

func (m *Manager) SendMessage(ctx context.Context, sessionID string, message []byte) error {
	state, has := m.GetSession(sessionID)
	if !has {
		return pkg.ErrLackSession
	}
	return state.sendMessage(ctx, message)
}

func (m *Manager) GetMessageForSend(ctx context.Context, sessionID string) ([]byte, error) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, pkg.ErrLackSession
	}
	return state.getMessageForSend(ctx)
}

func (m *Manager) UpdateSessionLastActiveAt(sessionID string) {
	state, ok := m.sessions.Load(sessionID)
	if !ok {
		return
	}
	state.updateLastActiveAt()
}

func (m *Manager) CloseSession(sessionID string) {
	state, ok := m.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	state.Close()
}

func (m *Manager) CheckSessionReady(sessionID string) bool {
	state, ok := m.sessions.Load(sessionID)
	if !ok {
		return false
	}
	return state.GetReady()
}

func (m *Manager) CloseAllSessions() {
	m.sessions.Range(func(sessionID string, _ *State) bool {
		// Here we load the session again to prevent concurrency conflicts with CloseSession, which may cause repeated close chan
		state, ok := m.sessions.LoadAndDelete(sessionID)
		if !ok {
			return true
		}
		state.Close()
		return true
	})
}

func (m *Manager) StartHeartbeatAndCleanInvalidSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	select {
	case <-m.stopHeartbeat:
		return
	case <-ticker.C:
		now := time.Now()
		m.sessions.Range(func(sessionID string, state *State) bool {
			if !m.CheckSessionReady(sessionID) {
				m.CloseSession(sessionID)
				return true
			}
			if m.maxIdleTime != 0 && now.Sub(state.lastActiveAt) > m.maxIdleTime {
				m.CloseSession(sessionID)
				return true
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			for i := 0; i < 3; i++ {
				if err := m.detection(ctx, sessionID); err == nil {
					return true
				}
			}
			m.CloseSession(sessionID)
			return true
		})
	}
}

func (m *Manager) StopHeartbeat() {
	close(m.stopHeartbeat)
}

func (m *Manager) RangeSessions(f func(sessionID string, state *State) bool) {
	m.sessions.Range(f)
}

func (m *Manager) IsEmpty() bool {
	isEmpty := true
	m.sessions.Range(func(string, *State) bool {
		isEmpty = false
		return false
	})
	return isEmpty
}
