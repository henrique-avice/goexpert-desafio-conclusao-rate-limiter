package strategy

import (
	"context"
	"testing"
)

func TestMockStrategyBasic(t *testing.T) {
	ctx := context.Background()
	mock := NewMockStrategy()

	// Deveria incrementar normalmente
	count, err := mock.CheckAndIncrement(ctx, "test")
	if count != 1 || err != nil {
		t.Fatalf("esperado count=1, err=nil; got count=%d, err=%v", count, err)
	}

	// Segundo incremento
	count, err = mock.CheckAndIncrement(ctx, "test")
	if count != 2 || err != nil {
		t.Fatalf("esperado count=2, err=nil; got count=%d, err=%v", count, err)
	}
}

func TestMockStrategyBlock(t *testing.T) {
	ctx := context.Background()
	mock := NewMockStrategy()

	// Ativar bloqueio
	mock.SetShouldBlock(true)

	// IsBlocked deveria retornar true
	blocked, err := mock.IsBlocked(ctx, "test")
	if !blocked || err != nil {
		t.Fatalf("esperado blocked=true; got blocked=%v, err=%v", blocked, err)
	}

	// CheckAndIncrement deveria retornar erro
	_, err = mock.CheckAndIncrement(ctx, "test")
	if err != ErrBlocked {
		t.Fatalf("esperado ErrBlocked; got err=%v", err)
	}
}

func TestMockStrategyReset(t *testing.T) {
	ctx := context.Background()
	mock := NewMockStrategy()

	// Incrementar
	mock.CheckAndIncrement(ctx, "test")
	mock.CheckAndIncrement(ctx, "test")

	// Verificar call count
	if mock.GetCallCount() != 2 {
		t.Fatalf("esperado callCount=2; got %d", mock.GetCallCount())
	}

	// Reset
	err := mock.Reset(ctx, "test")
	if err != nil {
		t.Fatalf("reset deveria suceder; got err=%v", err)
	}

	// Deveria estar resetado
	if mock.GetCallCount() != 0 {
		t.Fatalf("após reset, esperado callCount=0; got %d", mock.GetCallCount())
	}

	blocked, _ := mock.IsBlocked(ctx, "test")
	if blocked {
		t.Fatal("após reset, IsBlocked deveria retornar false")
	}
}
