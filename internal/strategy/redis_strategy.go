package strategy

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStrategy implementa RateLimitStrategy usando Redis como backend.
type RedisStrategy struct {
	client *redis.Client
	prefix string
	checkAndIncrScript *redis.Script
}

// Lua script atômico para CheckAndIncrement
var checkAndIncrLua = `
local blockKey = KEYS[1]
local counterKey = KEYS[2]
local ttl = tonumber(ARGV[1])

-- Verificar bloqueio
if redis.call('exists', blockKey) == 1 then
	return {err = 'BLOCKED'}
end

-- Incrementar contador
local count = redis.call('incr', counterKey)

-- Se é primeira requisição, setar TTL
if count == 1 then
	redis.call('expire', counterKey, ttl)
end

return count
`

// NewRedisStrategy cria nova estratégia Redis.
// addr: "localhost:6379"
// db: número do banco de dados (0-15)
// prefix: prefixo das chaves (ex: "rate_limit")
func NewRedisStrategy(addr string, db int, prefix string) (*RedisStrategy, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validar conexão
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}

	return &RedisStrategy{
		client:            client,
		prefix:            prefix,
		checkAndIncrScript: redis.NewScript(checkAndIncrLua),
	}, nil
}

// CheckAndIncrement verifica bloqueio, incrementa contador, aplica TTL (1 segundo).
// Usa Lua script para garantir atomicidade.
func (rs *RedisStrategy) CheckAndIncrement(ctx context.Context, identifier string) (int, error) {
	blockKey := fmt.Sprintf("%s:%s:block", rs.prefix, identifier)
	counterKey := fmt.Sprintf("%s:%s", rs.prefix, identifier)

	// Executar script Lua atomicamente
	result, err := rs.checkAndIncrScript.Run(ctx, rs.client,
		[]string{blockKey, counterKey},
		1, // TTL de 1 segundo
	).Result()

	if err != nil {
		return 0, fmt.Errorf("redis script execution failed: %w", err)
	}

	// Verificar se foi bloqueado
	if resultMap, ok := result.(map[string]interface{}); ok {
		if _, exists := resultMap["err"]; exists {
			return 0, ErrBlocked
		}
	}

	// Converter resultado para int
	count, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected script result type: %T", result)
	}

	return int(count), nil
}

// IsBlocked verifica se identifier está bloqueado.
func (rs *RedisStrategy) IsBlocked(ctx context.Context, identifier string) (bool, error) {
	blockKey := fmt.Sprintf("%s:%s:block", rs.prefix, identifier)
	exists, err := rs.client.Exists(ctx, blockKey).Result()
	if err != nil {
		return false, fmt.Errorf("redis check block failed: %w", err)
	}
	return exists > 0, nil
}

// Block marca identifier como bloqueado.
func (rs *RedisStrategy) Block(ctx context.Context, identifier string, blockDuration time.Duration) error {
	blockKey := fmt.Sprintf("%s:%s:block", rs.prefix, identifier)
	err := rs.client.Set(ctx, blockKey, "1", blockDuration).Err()
	if err != nil {
		return fmt.Errorf("redis set block failed: %w", err)
	}
	return nil
}

// Reset limpa contador e bloqueio.
func (rs *RedisStrategy) Reset(ctx context.Context, identifier string) error {
	counterKey := fmt.Sprintf("%s:%s", rs.prefix, identifier)
	blockKey := fmt.Sprintf("%s:%s:block", rs.prefix, identifier)

	err := rs.client.Del(ctx, counterKey, blockKey).Err()
	if err != nil {
		return fmt.Errorf("redis delete failed: %w", err)
	}
	return nil
}

// HealthCheck valida se Redis está acessível.
func (rs *RedisStrategy) HealthCheck(ctx context.Context) error {
	return rs.client.Ping(ctx).Err()
}

// Close fecha conexão com Redis.
func (rs *RedisStrategy) Close() error {
	return rs.client.Close()
}

// FlushDB limpa todo o banco Redis (útil para testes).
func (rs *RedisStrategy) FlushDB(ctx context.Context) error {
	return rs.client.FlushDB(ctx).Err()
}

// GetCount retorna o contador atual (útil para testes).
func (rs *RedisStrategy) GetCount(ctx context.Context, identifier string) (int, error) {
	counterKey := fmt.Sprintf("%s:%s", rs.prefix, identifier)
	val, err := rs.client.Get(ctx, counterKey).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("redis get failed: %w", err)
	}
	count, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid counter value: %w", err)
	}
	return count, nil
}

