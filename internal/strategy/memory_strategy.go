package strategy

import (
	"context"
	"sync"
	"time"
)

// MemoryStrategy implementa RateLimitStrategy usando map em-memória.
// Útil para testes e ambientes sem Redis.
type MemoryStrategy struct {
	counters map[string]*CounterData
	blocks   map[string]time.Time
	mu       sync.RWMutex
	done     chan struct{}
	wg       sync.WaitGroup // Supervisor para goroutine de cleanup
}

// CounterData armazena contador com TTL.
type CounterData struct {
	Count     int
	ExpiresAt time.Time
}

// NewMemoryStrategy cria nova estratégia em-memória com cleanup automático.
func NewMemoryStrategy() *MemoryStrategy {
	ms := &MemoryStrategy{
		counters: make(map[string]*CounterData),
		blocks:   make(map[string]time.Time),
		done:     make(chan struct{}),
	}

	// Goroutine de limpeza a cada 500ms com supervisor WaitGroup
	ms.wg.Add(1)
	go ms.cleanup()

	return ms
}

// CheckAndIncrement verifica bloqueio, incrementa contador, aplica TTL.
func (ms *MemoryStrategy) CheckAndIncrement(ctx context.Context, identifier string) (int, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if blockTime, exists := ms.blocks[identifier]; exists {
		if time.Now().Before(blockTime) {
			return 0, ErrBlocked
		}
		delete(ms.blocks, identifier)
	}

	counter := ms.counters[identifier]
	if counter == nil || counter.ExpiresAt.Before(time.Now()) {
		counter = &CounterData{Count: 0}
	}

	counter.Count++

	if counter.Count == 1 {
		counter.ExpiresAt = time.Now().Add(1 * time.Second)
	}

	ms.counters[identifier] = counter
	return counter.Count, nil
}

// IsBlocked verifica se identifier está bloqueado.
func (ms *MemoryStrategy) IsBlocked(ctx context.Context, identifier string) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	blockTime, exists := ms.blocks[identifier]
	if !exists {
		return false, nil
	}

	return time.Now().Before(blockTime), nil
}

// Block marca identifier como bloqueado.
func (ms *MemoryStrategy) Block(ctx context.Context, identifier string, blockDuration time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	unblockTime := time.Now().Add(blockDuration)
	ms.blocks[identifier] = unblockTime
	return nil
}

// Reset limpa contador e bloqueio.
func (ms *MemoryStrategy) Reset(ctx context.Context, identifier string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.counters, identifier)
	delete(ms.blocks, identifier)
	return nil
}

// HealthCheck sempre retorna nil (memória sempre está disponível).
func (ms *MemoryStrategy) HealthCheck(ctx context.Context) error {
	return nil
}

// Close encerra a estratégia e para goroutine de cleanup com WaitGroup.
func (ms *MemoryStrategy) Close() error {
	close(ms.done)
	ms.wg.Wait() // Aguardar cleanup goroutine terminar
	return nil
}

// cleanup remove entries expiradas periodicamente com WaitGroup.
func (ms *MemoryStrategy) cleanup() {
	defer ms.wg.Done() // Notificar WaitGroup ao sair

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ms.done:
			return
		case <-ticker.C:
			ms.mu.Lock()
			now := time.Now()
			for key, counter := range ms.counters {
				if counter.ExpiresAt.Before(now) {
					delete(ms.counters, key)
				}
			}
			for key, blockTime := range ms.blocks {
				if blockTime.Before(now) {
					delete(ms.blocks, key)
				}
			}
			ms.mu.Unlock()
		}
	}
}

