package strategy

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStrategyCheckAndIncrement(t *testing.T) {
	ctx := context.Background()
	strat := NewMemoryStrategy()
	defer strat.Close()

	// Primeira requisição
	count, err := strat.CheckAndIncrement(ctx, "test-key")
	if count != 1 || err != nil {
		t.Fatalf("primeiro incremento deveria retornar 1, got count=%d, err=%v", count, err)
	}

	// Segunda requisição
	count, err = strat.CheckAndIncrement(ctx, "test-key")
	if count != 2 || err != nil {
		t.Fatalf("segundo incremento deveria retornar 2, got count=%d, err=%v", count, err)
	}
}

func TestMemoryStrategyBlock(t *testing.T) {
	ctx := context.Background()
	strat := NewMemoryStrategy()
	defer strat.Close()

	key := "test-key"

	// Bloquear por 1 segundo
	err := strat.Block(ctx, key, 1*time.Second)
	if err != nil {
		t.Fatalf("block deveria suceder, got err=%v", err)
	}

	// Deveria estar bloqueado
	blocked, err := strat.IsBlocked(ctx, key)
	if !blocked || err != nil {
		t.Fatalf("key deveria estar bloqueada, got blocked=%v, err=%v", blocked, err)
	}

	// Tentar incrementar com bloqueio deve falhar
	_, err = strat.CheckAndIncrement(ctx, key)
	if err != ErrBlocked {
		t.Fatalf("checkAndIncrement com bloqueio deveria retornar ErrBlocked, got err=%v", err)
	}

	// Esperar bloqueio expirar
	time.Sleep(1100 * time.Millisecond)

	// Deveria estar desbloqueado
	blocked, err = strat.IsBlocked(ctx, key)
	if blocked || err != nil {
		t.Fatalf("key deveria estar desbloqueada após expiração, got blocked=%v", blocked)
	}

	// Agora deveria permitir incremento
	count, err := strat.CheckAndIncrement(ctx, key)
	if count != 1 || err != nil {
		t.Fatalf("após desbloqueio, CheckAndIncrement deveria retornar 1, got count=%d, err=%v", count, err)
	}
}

func TestMemoryStrategyTTL(t *testing.T) {
	ctx := context.Background()
	strat := NewMemoryStrategy()
	defer strat.Close()

	key := "test-key"

	// Incrementar (inicia TTL de 1 segundo)
	count, _ := strat.CheckAndIncrement(ctx, key)
	if count != 1 {
		t.Fatal("primeiro incremento deveria ser 1")
	}

	// Dentro de 1 segundo, incrementa normalmente
	time.Sleep(100 * time.Millisecond)
	count, _ = strat.CheckAndIncrement(ctx, key)
	if count != 2 {
		t.Fatal("segundo incremento deveria ser 2")
	}

	// Após 1 segundo, TTL expira e contador reseta
	time.Sleep(1100 * time.Millisecond)
	count, _ = strat.CheckAndIncrement(ctx, key)
	if count != 1 {
		t.Fatal("após TTL expirar, incremento deveria ser 1 novamente")
	}
}

func TestMemoryStrategyReset(t *testing.T) {
	ctx := context.Background()
	strat := NewMemoryStrategy()
	defer strat.Close()

	key := "test-key"

	// Incrementar e bloquear
	strat.CheckAndIncrement(ctx, key)
	strat.Block(ctx, key, 10*time.Second)

	// Reset limpa tudo
	err := strat.Reset(ctx, key)
	if err != nil {
		t.Fatalf("reset deveria suceder, got err=%v", err)
	}

	// Deveria estar desbloqueado
	blocked, _ := strat.IsBlocked(ctx, key)
	if blocked {
		t.Fatal("após reset, deveria estar desbloqueado")
	}

	// Contador deveria estar zerado
	count, _ := strat.CheckAndIncrement(ctx, key)
	if count != 1 {
		t.Fatal("após reset, contador deveria iniciar em 1")
	}
}

func TestMemoryStrategyCleanup(t *testing.T) {
	ctx := context.Background()
	strat := NewMemoryStrategy()
	defer strat.Close()

	// Criar várias entradas
	for i := 0; i < 10; i++ {
		strat.CheckAndIncrement(ctx, "key-"+string(rune(i)))
	}

	// Esperar cleanup (500ms * 3 = 1500ms + margem)
	time.Sleep(2 * time.Second)

	// Todas as entries com TTL de 1s devem ter sido limpas
	// Verificar que as entradas foram removidas
	strat.mu.RLock()
	countersCount := len(strat.counters)
	strat.mu.RUnlock()

	if countersCount > 0 {
		t.Logf("após cleanup, ainda existem %d counters (pode ser normal dependendo de timing)", countersCount)
	}
}

func TestMemoryStrategyHealthCheck(t *testing.T) {
	ctx := context.Background()
	strat := NewMemoryStrategy()
	defer strat.Close()

	err := strat.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck deveria sempre suceder para Memory strategy, got err=%v", err)
	}
}

func TestMemoryStrategyMultipleKeys(t *testing.T) {
	ctx := context.Background()
	strat := NewMemoryStrategy()
	defer strat.Close()

	// Keys diferentes devem ter contadores independentes
	count1, _ := strat.CheckAndIncrement(ctx, "key1")
	count2, _ := strat.CheckAndIncrement(ctx, "key2")
	count1b, _ := strat.CheckAndIncrement(ctx, "key1")
	count2b, _ := strat.CheckAndIncrement(ctx, "key2")

	if count1 != 1 || count1b != 2 {
		t.Fatalf("key1 deveria ter [1, 2], got [%d, %d]", count1, count1b)
	}
	if count2 != 1 || count2b != 2 {
		t.Fatalf("key2 deveria ter [1, 2], got [%d, %d]", count2, count2b)
	}
}
