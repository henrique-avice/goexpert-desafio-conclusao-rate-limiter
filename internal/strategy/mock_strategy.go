package strategy

import (
	"context"
	"sync"
	"time"
)

// MockStrategy é uma strategy para testes que permite controlar comportamento.
type MockStrategy struct {
	shouldBlock bool
	callCount   int
	mu          sync.Mutex
}

// NewMockStrategy cria strategy para testes.
func NewMockStrategy() *MockStrategy {
	return &MockStrategy{
		shouldBlock: false,
		callCount:   0,
	}
}

// CheckAndIncrement simula incremento, retorna erro se shouldBlock.
func (m *MockStrategy) CheckAndIncrement(ctx context.Context, identifier string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++

	if m.shouldBlock {
		return 0, ErrBlocked
	}

	return m.callCount, nil
}

// IsBlocked retorna shouldBlock.
func (m *MockStrategy) IsBlocked(ctx context.Context, identifier string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.shouldBlock, nil
}

// Block seta shouldBlock = true.
func (m *MockStrategy) Block(ctx context.Context, identifier string, blockDuration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.shouldBlock = true
	return nil
}

// Reset limpa estado.
func (m *MockStrategy) Reset(ctx context.Context, identifier string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.shouldBlock = false
	m.callCount = 0
	return nil
}

// HealthCheck sempre sucede.
func (m *MockStrategy) HealthCheck(ctx context.Context) error {
	return nil
}

// Close é no-op para mock.
func (m *MockStrategy) Close() error {
	return nil
}

// SetShouldBlock permite controlar resposta.
func (m *MockStrategy) SetShouldBlock(block bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.shouldBlock = block
}

// GetCallCount retorna quantas vezes CheckAndIncrement foi chamado.
func (m *MockStrategy) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.callCount
}
