package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/henrique-avice/goexpert-desafio-conclusao-rate-limiter/internal/config"
	"github.com/henrique-avice/goexpert-desafio-conclusao-rate-limiter/internal/handlers"
	"github.com/henrique-avice/goexpert-desafio-conclusao-rate-limiter/internal/limiter"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Carregar config
	cfg := config.NewConfig()

	// Criar strategy com fallback automático
	strat, err := cfg.CreateStrategy()
	if err != nil {
		logger.Error("failed to create strategy", "error", err)
		os.Exit(1)
	}
	defer strat.Close()

	// Criar limiter
	rateLimiter := limiter.NewRateLimiter(
		strat,
		cfg.RateLimitIPRequests,
		cfg.RateLimitTokenRequests,
		cfg.RateLimitBlockTime,
	)
	defer rateLimiter.Close()

	// Criar middleware
	rateLimitMiddleware := limiter.NewRateLimitMiddleware(rateLimiter)

	// Criar router
	router := chi.NewRouter()

	// Middlewares globais
	router.Use(middleware.Logger)
	router.Use(rateLimitMiddleware.Handler)

	// Rotas
	router.Get("/api/test", handlers.TestHandler)
	router.Get("/api/users", handlers.UsersHandler)
	router.Post("/api/submit", handlers.SubmitHandler)

	// Health check (sem rate limit seria ideal, mas spec não menciona)
	router.Get("/health", handlers.HealthHandler)

	// Iniciar server
	addr := ":" + cfg.ServerPort
	logger.Info("Rate Limiter server starting",
		"address", "http://localhost:"+cfg.ServerPort,
		"ip_limit", cfg.RateLimitIPRequests,
		"token_limit", cfg.RateLimitTokenRequests,
		"block_time_seconds", cfg.RateLimitBlockTime)

	if err := http.ListenAndServe(addr, router); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
