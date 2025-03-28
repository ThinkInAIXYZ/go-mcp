package transport

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockSessionManager是一个简单的TransportSessionManager实现，用于测试
type MockSessionManager struct {
	sessions sync.Map // key=sessionID, value=chan []byte
}

// CreateSession实现TransportSessionManager接口
func (m *MockSessionManager) CreateSession() (string, chan []byte) {
	sessionID := "test-session-id"
	sessionChan := make(chan []byte, 64)
	m.sessions.Store(sessionID, sessionChan)
	return sessionID, sessionChan
}

// GetSessionChan实现TransportSessionManager接口
func (m *MockSessionManager) GetSessionChan(sessionID string) (chan []byte, bool) {
	value, ok := m.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	ch, ok := value.(chan []byte)
	return ch, ok
}

// CloseSession实现TransportSessionManager接口
func (m *MockSessionManager) CloseSession(sessionID string) {
	value, ok := m.sessions.Load(sessionID)
	if !ok {
		return
	}
	ch, ok := value.(chan []byte)
	if ok {
		close(ch)
	}
	m.sessions.Delete(sessionID)
}

type serverReceive func(ctx context.Context, sessionID string, msg []byte) error

func (r serverReceive) Receive(ctx context.Context, sessionID string, msg []byte) error {
	return r(ctx, sessionID, msg)
}

type clientReceive func(ctx context.Context, msg []byte) error

func (r clientReceive) Receive(ctx context.Context, msg []byte) error {
	return r(ctx, msg)
}

func testTransport(t *testing.T, client ClientTransport, server ServerTransport) {
	msgWithServer := "hello"
	expectedMsgWithServer := ""
	server.SetReceiver(serverReceive(func(ctx context.Context, sessionID string, msg []byte) error {
		expectedMsgWithServer = string(msg)
		return nil
	}))

	// 设置MockSessionManager用于测试
	mockSessionManager := &MockSessionManager{}
	server.SetSessionManager(mockSessionManager)

	msgWithClient := "hello"
	expectedMsgWithClient := ""
	client.SetReceiver(clientReceive(func(ctx context.Context, msg []byte) error {
		expectedMsgWithClient = string(msg)
		return nil
	}))

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run()
	}()

	// 使用 select 来处理可能的错误
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Run() failed: %v", err)
		}
	case <-time.After(time.Second):
		// 服务器正常启动
	}

	defer func() {
		userCtx, cancel := context.WithTimeout(context.Background(), time.Second*1)
		defer cancel()

		serverCtx, cancel := context.WithCancel(userCtx)
		cancel()

		if err := server.Shutdown(userCtx, serverCtx); err != nil {
			t.Errorf("server.Shutdown() failed: %v", err)
		}
	}()

	if err := client.Start(); err != nil {
		t.Fatalf("client.Start() failed: %v", err)
	}

	defer func() {
		if err := client.Close(context.Background()); err != nil {
			t.Errorf("client.Close() failed: %v", err)
		}
	}()

	if err := client.Send(context.Background(), Message(msgWithServer)); err != nil {
		t.Fatalf("client.Send() failed: %v", err)
	}
	time.Sleep(time.Second * 1)
	assert.Equal(t, expectedMsgWithServer, msgWithServer)

	sessionID := ""
	if cli, ok := client.(*SSEClientTransport); ok {
		sessionID = cli.messageEndpoint.Query().Get("sessionID")
	}

	if err := server.Send(context.Background(), sessionID, Message(msgWithClient)); err != nil {
		t.Fatalf("server.Send() failed: %v", err)
	}
	time.Sleep(time.Second * 1)
	assert.Equal(t, expectedMsgWithClient, msgWithClient)
}
