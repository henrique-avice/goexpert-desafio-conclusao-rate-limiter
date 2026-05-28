package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/henrique-avice/goexpert-desafio-conclusao-rate-limiter/internal/strategy"
)

func TestIPRateLimit(t *testing.T) {
	ctx := context.Background()
	strat := strategy.NewMemoryStrategy()
	defer strat.Close()

	limiter := NewRateLimiter(strat, 3, 100, 5)

	// 3 requisições de mesmo IP devem ser permitidas
	for i := 1; i <= 3; i++ {
		allowed, blocked, _ := limiter.Allow(ctx, "192.168.1.1", false)
		if !allowed || blocked {
			t.Fatalf("requisição %d deveria ser permitida, got allowed=%v, blocked=%v", i, allowed, blocked)
		}
	}

	// 4ª requisição deve ser bloqueada
	allowed, blocked, _ := limiter.Allow(ctx, "192.168.1.1", false)
	if allowed || !blocked {
		t.Fatalf("4ª requisição deveria ser bloqueada, got allowed=%v, blocked=%v", allowed, blocked)
	}
}

func TestTokenRateLimit(t *testing.T) {
	ctx := context.Background()
	strat := strategy.NewMemoryStrategy()
	defer strat.Close()

	limiter := NewRateLimiter(strat, 10, 5, 5)

	// 5 requisições com token devem ser permitidas
	for i := 1; i <= 5; i++ {
		allowed, blocked, _ := limiter.Allow(ctx, "token-123", true)
		if !allowed || blocked {
			t.Fatalf("requisição %d deveria ser permitida, got allowed=%v, blocked=%v", i, allowed, blocked)
		}
	}

	// 6ª requisição deve ser bloqueada (token limit é 5)
	allowed, blocked, _ := limiter.Allow(ctx, "token-123", true)
	if allowed || !blocked {
		t.Fatalf("6ª requisição deveria ser bloqueada, got allowed=%v, blocked=%v", allowed, blocked)
	}
}

func TestTokenPrecedenceOverIP(t *testing.T) {
	ctx := context.Background()
	strat := strategy.NewMemoryStrategy()
	defer strat.Close()

	limiter := NewRateLimiter(strat, 2, 10, 5)

	// Mesmo IP, mas COM token: usar token limit (10), não IP limit (2)
	for i := 1; i <= 10; i++ {
		allowed, blocked, _ := limiter.Allow(ctx, "client-token-xyz", true)
		if !allowed || blocked {
			t.Fatalf("requisição %d com token deveria ser permitida (token limit=10), got blocked", i)
		}
	}

	// 11ª requisição com token deve ser bloqueada
	allowed, blocked, _ := limiter.Allow(ctx, "client-token-xyz", true)
	if allowed || !blocked {
		t.Fatal("11ª requisição com token deveria ser bloqueada")
	}

	// IP SEM token: deveria ter limit=2 (independente do histórico do token)
	limiter.Reset(ctx, "192.168.1.50", false)
	for i := 1; i <= 2; i++ {
		allowed, blocked, _ := limiter.Allow(ctx, "192.168.1.50", false)
		if !allowed || blocked {
			t.Fatalf("requisição %d do IP deveria ser permitida (IP limit=2)", i)
		}
	}

	// 3ª requisição do IP deve ser bloqueada
	allowed, blocked, _ = limiter.Allow(ctx, "192.168.1.50", false)
	if allowed || !blocked {
		t.Fatal("3ª requisição do IP deveria ser bloqueada (IP limit=2)")
	}
}

func TestBlockDuration(t *testing.T) {
	ctx := context.Background()
	strat := strategy.NewMemoryStrategy()
	defer strat.Close()

	limiter := NewRateLimiter(strat, 1, 100, 1) // block time = 1 segundo

	// 1ª requisição permitida
	allowed, blocked, _ := limiter.Allow(ctx, "192.168.1.1", false)
	if !allowed || blocked {
		t.Fatal("1ª requisição deveria ser permitida")
	}

	// 2ª requisição bloqueada (excede limit de 1)
	allowed, blocked, _ = limiter.Allow(ctx, "192.168.1.1", false)
	if allowed || !blocked {
		t.Fatal("2ª requisição deveria ser bloqueada")
	}

	// Imediatamente após, ainda deve estar bloqueado
	blocked, _ = strat.IsBlocked(ctx, "ip:192.168.1.1")
	if !blocked {
		t.Fatal("deveria estar bloqueado imediatamente")
	}

	// Esperar bloqueio expirar
	time.Sleep(1100 * time.Millisecond)

	// Após expiração, deveria estar desbloqueado
	blocked, _ = strat.IsBlocked(ctx, "ip:192.168.1.1")
	if blocked {
		t.Fatal("deveria estar desbloqueado após expiração")
	}

	// Nova requisição deveria ser permitida (contador reset)
	allowed, blocked, _ = limiter.Allow(ctx, "192.168.1.1", false)
	if !allowed || blocked {
		t.Fatal("requisição após desbloqueio deveria ser permitida")
	}
}

func TestMultipleIPsIsolated(t *testing.T) {
	ctx := context.Background()
	strat := strategy.NewMemoryStrategy()
	defer strat.Close()

	limiter := NewRateLimiter(strat, 2, 100, 5)

	// IP1: 2 requisições
	for i := 1; i <= 2; i++ {
		allowed, _, _ := limiter.Allow(ctx, "192.168.1.1", false)
		if !allowed {
			t.Fatalf("IP1 req %d deveria ser permitida", i)
		}
	}

	// IP2: 2 requisições (independente de IP1)
	for i := 1; i <= 2; i++ {
		allowed, _, _ := limiter.Allow(ctx, "192.168.1.2", false)
		if !allowed {
			t.Fatalf("IP2 req %d deveria ser permitida", i)
		}
	}

	// IP1: 3ª requisição bloqueada
	allowed, blocked, _ := limiter.Allow(ctx, "192.168.1.1", false)
	if allowed || !blocked {
		t.Fatal("IP1 3ª req deveria ser bloqueada")
	}

	// IP2: 3ª requisição também bloqueada
	allowed, blocked, _ = limiter.Allow(ctx, "192.168.1.2", false)
	if allowed || !blocked {
		t.Fatal("IP2 3ª req deveria ser bloqueada")
	}
}

func TestCounterExpiry(t *testing.T) {
	ctx := context.Background()
	strat := strategy.NewMemoryStrategy()
	defer strat.Close()

	limiter := NewRateLimiter(strat, 2, 100, 1) // block time = 1 segundo

	// 2 requisições
	for i := 1; i <= 2; i++ {
		allowed, _, _ := limiter.Allow(ctx, "192.168.1.1", false)
		if !allowed {
			t.Fatalf("requisição %d deveria ser permitida", i)
		}
	}

	// 3ª requisição bloqueada
	allowed, _, _ := limiter.Allow(ctx, "192.168.1.1", false)
	if allowed {
		t.Fatal("3ª requisição deveria ser bloqueada")
	}

	// Esperar contador + bloqueio expirarem (TTL=1s + block=1s + margem)
	time.Sleep(2100 * time.Millisecond)

	// Contador e bloqueio devem estar zerados, agora permite novamente
	allowed, _, _ = limiter.Allow(ctx, "192.168.1.1", false)
	if !allowed {
		t.Fatal("após expiração do contador e bloqueio, deveria permitir novamente")
	}
}

func TestReset(t *testing.T) {
	ctx := context.Background()
	strat := strategy.NewMemoryStrategy()
	defer strat.Close()

	limiter := NewRateLimiter(strat, 1, 100, 5)

	// 1 requisição
	limiter.Allow(ctx, "192.168.1.1", false)

	// 2ª bloqueada
	allowed, _, _ := limiter.Allow(ctx, "192.168.1.1", false)
	if allowed {
		t.Fatal("2ª req deveria ser bloqueada")
	}

	// Reset limpa estado
	limiter.Reset(ctx, "192.168.1.1", false)

	// Agora permite novamente
	allowed, _, _ = limiter.Allow(ctx, "192.168.1.1", false)
	if !allowed {
		t.Fatal("após reset, deveria permitir")
	}
}
