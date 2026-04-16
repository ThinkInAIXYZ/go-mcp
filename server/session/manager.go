package session

import (
	"context"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type Manager struct {
	activeSessions pkg.SyncMap[*State]
	closedSessions pkg.SyncMap[struct{}]

	stopHeartbeat chan struct{}

	genSessionID func(ctx context.Context) string

	logger pkg.Logger

	detection   func(ctx context.Context, sessionID string) error
	maxIdleTime time.Duration
}

func NewManager(detection func(ctx context.Context, sessionID string) error, genSessionID func(ctx context.Context) string) *Manager {
	return &Manager{
		genSessionID:  genSessionID,
		detection:     detection,
		stopHeartbeat: make(chan struct{}),
		logger:        pkg.DefaultLogger,
	}
}

func (m *Manager) SetMaxIdleTime(d time.Duration) {
	m.maxIdleTime = d
}

func (m *Manager) SetLogger(logger pkg.Logger) {
	m.logger = logger
}

func (m *Manager) CreateSession(ctx context.Context) string {
	sessionID := m.genSessionID(ctx)
	state := NewState()
	m.activeSessions.Store(sessionID, state)
	return sessionID
}

func (m *Manager) IsActiveSession(sessionID string) bool {
	_, has := m.activeSessions.Load(sessionID)
	return has
}

func (m *Manager) IsClosedSession(sessionID string) bool {
	_, has := m.closedSessions.Load(sessionID)
	return has
}

func (m *Manager) GetSession(sessionID string) (*State, bool) {
	if sessionID == "" {
		return nil, false
	}
	state, has := m.activeSessions.Load(sessionID)
	if !has {
		return nil, false
	}
	return state, true
}

func (m *Manager) OpenMessageQueueForSend(sessionID string) error {
	state, has := m.GetSession(sessionID)
	if !has {
		return pkg.ErrLackSession
	}
	state.openMessageQueueForSend()
	return nil
}

func (m *Manager) EnqueueMessageForSend(ctx context.Context, sessionID string, message []byte) error {
	state, has := m.GetSession(sessionID)
	if !has {
		return pkg.ErrLackSession
	}
	return state.enqueueMessage(ctx, message)
}

func (m *Manager) DequeueMessageForSend(ctx context.Context, sessionID string) ([]byte, error) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, pkg.ErrLackSession
	}
	return state.dequeueMessage(ctx)
}

func (m *Manager) UpdateSessionLastActiveAt(sessionID string) {
	state, ok := m.activeSessions.Load(sessionID)
	if !ok {
		return
	}
	state.updateLastActiveAt()
}

func (m *Manager) CloseSession(sessionID string) {
	state, ok := m.activeSessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	state.Close()
	m.closedSessions.Store(sessionID, struct{}{})
}

func (m *Manager) CloseAllSessions() {
	m.activeSessions.Range(func(sessionID string, _ *State) bool {
		// Here we load the session again to prevent concurrency conflicts with CloseSession, which may cause repeated close chan
		m.CloseSession(sessionID)
		return true
	})
}

func (m *Manager) StartHeartbeatAndCleanInvalidSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopHeartbeat:
			return
		case <-ticker.C:
			now := time.Now()
			m.activeSessions.Range(func(sessionID string, state *State) bool {
				if m.maxIdleTime != 0 && now.Sub(state.lastActiveAt) > m.maxIdleTime {
					m.logger.Infof("session expire, session id: %v", sessionID)
					m.CloseSession(sessionID)
					return true
				}

				var err error
				for i := 0; i < 3; i++ {
					if err = m.detection(context.Background(), sessionID); err == nil {
						return true
					}
				}
				m.logger.Infof("session detection fail, session id: %v, fail reason: %+v", sessionID, err)
				m.CloseSession(sessionID)
				return true
			})
		}
	}
}

func (m *Manager) StopHeartbeat() {
	close(m.stopHeartbeat)
}

func (m *Manager) RangeSessions(f func(sessionID string, state *State) bool) {
	m.activeSessions.Range(f)
}

func (m *Manager) IsEmpty() bool {
	isEmpty := true
	m.activeSessions.Range(func(string, *State) bool {
		isEmpty = false
		return false
	})
	return isEmpty
}
