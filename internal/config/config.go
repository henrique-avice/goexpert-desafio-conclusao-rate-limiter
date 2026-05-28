package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/henrique-avice/goexpert-desafio-conclusao-rate-limiter/internal/strategy"
)

// Config contém configurações da aplicação.
type Config struct {
	// Rate Limit
	RateLimitIPRequests    int
	RateLimitTokenRequests int
	RateLimitBlockTime     int // segundos

	// Redis
	RedisHost     string
	RedisPort     string
	RedisDB       int
	RedisPassword string

	// Backend
	RateLimitBackend string // "redis" ou "memory"

	// Server
	ServerPort string
	ServerEnv  string
}

// NewConfig carrega configuração do ambiente.
func NewConfig() *Config {
	cfg := &Config{
		// Rate Limit
		RateLimitIPRequests:    getEnvInt("RATE_LIMIT_IP_REQUESTS", 10),
		RateLimitTokenRequests: getEnvInt("RATE_LIMIT_TOKEN_REQUESTS", 100),
		RateLimitBlockTime:     getEnvInt("RATE_LIMIT_BLOCK_TIME", 300),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisDB:       getEnvInt("REDIS_DB", 0),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// Backend
		RateLimitBackend: getEnv("RATE_LIMIT_BACKEND", "redis"),

		// Server
		ServerPort: getEnv("SERVER_PORT", "8080"),
		ServerEnv:  getEnv("SERVER_ENV", "development"),
	}

	cfg.validate()
	return cfg
}

// CreateStrategy cria a strategy configurada com fallback automático.
func (c *Config) CreateStrategy() (strategy.RateLimitStrategy, error) {
	if c.RateLimitBackend == "memory" {
		return strategy.NewMemoryStrategy(), nil
	}

	// Tentar Redis
	addr := c.RedisHost + ":" + c.RedisPort
	strat, err := strategy.NewRedisStrategy(addr, c.RedisDB, "rate_limit")
	if err != nil {
		// Fallback para Memory se Redis falhar
		slog.Warn("Redis unavailable, using Memory strategy", "error", err)
		return strategy.NewMemoryStrategy(), nil
	}

	return strat, nil
}

// getEnv lê variável de ambiente com fallback.
func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvInt lê variável de ambiente como int com fallback.
func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

// validate valida valores de configuração.
func (c *Config) validate() {
	if c.RateLimitIPRequests < 1 {
		slog.Error("RATE_LIMIT_IP_REQUESTS deve ser >= 1")
		os.Exit(1)
	}

	if c.RateLimitTokenRequests < 1 {
		slog.Error("RATE_LIMIT_TOKEN_REQUESTS deve ser >= 1")
		os.Exit(1)
	}

	if c.RateLimitBlockTime < 1 {
		slog.Error("RATE_LIMIT_BLOCK_TIME deve ser >= 1")
		os.Exit(1)
	}

	if c.RateLimitBackend != "redis" && c.RateLimitBackend != "memory" {
		slog.Error("RATE_LIMIT_BACKEND deve ser 'redis' ou 'memory'")
		os.Exit(1)
	}
}
