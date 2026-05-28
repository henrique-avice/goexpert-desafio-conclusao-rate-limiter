package strategy

import (
	"context"
	"errors"
	"time"
)

// RateLimitStrategy define o contrato para diferentes implementações
// de armazenamento de contadores e estado de bloqueio.
type RateLimitStrategy interface {
	// CheckAndIncrement verifica se uma requisição é permitida e incrementa o contador.
	// Retorna o novo valor do contador ou erro.
	CheckAndIncrement(ctx context.Context, identifier string) (int, error)

	// IsBlocked verifica se uma chave está bloqueada.
	IsBlocked(ctx context.Context, identifier string) (bool, error)

	// Block marca uma chave como bloqueada por blockDuration.
	Block(ctx context.Context, identifier string, blockDuration time.Duration) error

	// Reset limpa contador e bloqueio de uma chave.
	Reset(ctx context.Context, identifier string) error

	// HealthCheck valida se a strategy está acessível.
	HealthCheck(ctx context.Context) error

	// Close encerra conexões e libera recursos.
	Close() error
}

var (
	ErrBlocked       = errors.New("identifier is blocked")
	ErrLimitExceeded = errors.New("rate limit exceeded")
)
