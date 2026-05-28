package limiter

import (
	"context"
	"sync"
	"time"

	"github.com/henrique-avice/goexpert-desafio-conclusao-rate-limiter/internal/strategy"
)

// RateLimiter orquestra verificação de rate limit com uma strategy.
type RateLimiter struct {
	strategy           strategy.RateLimitStrategy
	fallbackStrategy   strategy.RateLimitStrategy // Memory como fallback
	ipLimit            int
	tokenLimit         int
	blockTime          time.Duration
	healthCheckFailure int    // contador de falhas de health check
	maxFailures        int    // limite de falhas antes de usar fallback (padrão 3)
	usingFallback      bool   // flag: estamos usando fallback?
	mu                 sync.RWMutex // proteção para healthCheckFailure e usingFallback
}

// NewRateLimiter cria novo limitador.
// ipLimit: requisições por segundo para IP (padrão 10)
// tokenLimit: requisições por segundo para token (padrão 100)
// blockTime: tempo de bloqueio em segundos (padrão 300 = 5 min)
func NewRateLimiter(
	strat strategy.RateLimitStrategy,
	ipLimit int,
	tokenLimit int,
	blockTimeSeconds int,
) *RateLimiter {
	return &RateLimiter{
		strategy:         strat,
		fallbackStrategy: strategy.NewMemoryStrategy(),
		ipLimit:          ipLimit,
		tokenLimit:       tokenLimit,
		blockTime:        time.Duration(blockTimeSeconds) * time.Second,
		maxFailures:      3,
	}
}

// Allow verifica se requisição é permitida para identifier.
// tokenBased: true se é token, false se é IP.
// Retorna (allowed, blocked, error).
// Implementa fail-open: Redis down → Memory strategy.
func (rl *RateLimiter) Allow(ctx context.Context, identifier string, tokenBased bool) (bool, bool, error) {
	rl.tryRecoverPrimary(ctx)

	limit := rl.ipLimit
	if tokenBased {
		limit = rl.tokenLimit
	}

	prefix := "ip"
	if tokenBased {
		prefix = "token"
	}
	key := prefix + ":" + identifier

	currentStrategy := rl.getCurrentStrategy()
	usingFallback := rl.isUsingFallback()

	blocked, err := currentStrategy.IsBlocked(ctx, key)
	if err != nil {
		if !usingFallback {
			rl.incrementFailureCount()
		}
		return true, false, nil // fail-open
	}
	if blocked {
		return false, true, nil
	}

	count, err := currentStrategy.CheckAndIncrement(ctx, key)
	if err == strategy.ErrBlocked {
		return false, true, nil
	}
	if err != nil {
		if !usingFallback {
			rl.incrementFailureCount()
		}
		return true, false, nil // fail-open
	}

	if err == nil && usingFallback {
		rl.resetFailureCount()
	}

	if count > limit {
		_ = currentStrategy.Block(ctx, key, rl.blockTime)
		return false, true, nil
	}

	return true, false, nil
}

// getCurrentStrategy retorna a strategy atual (Redis ou fallback) de forma thread-safe.
func (rl *RateLimiter) getCurrentStrategy() strategy.RateLimitStrategy {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if rl.usingFallback {
		return rl.fallbackStrategy
	}
	return rl.strategy
}

// isUsingFallback retorna status de fallback de forma thread-safe.
func (rl *RateLimiter) isUsingFallback() bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.usingFallback
}

// incrementFailureCount incrementa contador de falhas e ativa fallback se necessário.
func (rl *RateLimiter) incrementFailureCount() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.healthCheckFailure++
	if rl.healthCheckFailure >= rl.maxFailures {
		rl.usingFallback = true
	}
}

// resetFailureCount reseta contador quando strategy volta online.
func (rl *RateLimiter) resetFailureCount() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.healthCheckFailure = 0
	rl.usingFallback = false
}

// Reset limpa estado de um identifier (útil para testes).
func (rl *RateLimiter) Reset(ctx context.Context, identifier string, tokenBased bool) error {
	prefix := "ip"
	if tokenBased {
		prefix = "token"
	}
	key := prefix + ":" + identifier

	return rl.strategy.Reset(ctx, key)
}

// HealthCheck valida se strategy está saudável.
func (rl *RateLimiter) HealthCheck(ctx context.Context) error {
	return rl.strategy.HealthCheck(ctx)
}

// Close encerra recursos.
func (rl *RateLimiter) Close() error {
	return rl.strategy.Close()
}

// GetBlockTime retorna o tempo de bloqueio configurado.
func (rl *RateLimiter) GetBlockTime() time.Duration {
	return rl.blockTime
}

// tryRecoverPrimary tenta recuperar a strategy primária quando em fallback.
func (rl *RateLimiter) tryRecoverPrimary(ctx context.Context) {
	if !rl.usingFallback {
		return
	}

	if err := rl.strategy.HealthCheck(ctx); err == nil {
		rl.resetFailureCount()
	}
}

