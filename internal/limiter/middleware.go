package limiter

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// RateLimitMiddleware é middleware HTTP de rate limiting.
type RateLimitMiddleware struct {
	limiter *RateLimiter
}

// NewRateLimitMiddleware cria novo middleware.
func NewRateLimitMiddleware(limiter *RateLimiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: limiter,
	}
}

// Handler retorna função middleware HTTP.
// Uso: chi.Use(middleware.Handler)
func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ip := m.extractIP(r)

		token := r.Header.Get("API_KEY")
		tokenPresent := token != ""

		var identifier string
		if tokenPresent {
			identifier = token
		} else {
			identifier = ip
		}

		allowed, _, _ := m.limiter.Allow(ctx, identifier, tokenPresent)

		if !allowed {
			m.respond429(w, m.limiter.GetBlockTime())
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP extrai o IP do cliente priorizando X-Forwarded-For, X-Real-IP e RemoteAddr.
func (m *RateLimitMiddleware) extractIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		ip := strings.TrimSpace(ips[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		realIP = strings.TrimSpace(realIP)
		if net.ParseIP(realIP) != nil {
			return realIP
		}
	}

	if r.RemoteAddr != "" {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			return host
		}
		return strings.Trim(r.RemoteAddr, "[]")
	}

	// Gera identificador aleatório para evitar colisão no bucket "unknown"
	return generateSecureUnknownIP()
}

// generateSecureUnknownIP gera um identificador aleatório seguro usando crypto/rand.
// Formato: "unknown-{16 hex characters}"
func generateSecureUnknownIP() string {
	b := make([]byte, 8) // 8 bytes = 16 caracteres hex
	_, err := rand.Read(b)
	if err != nil {
		// Fallback: usar padrão com timestamp se algo falhar
		return fmt.Sprintf("unknown-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("unknown-%x", b)
}

// respond429 retorna HTTP 429 com mensagem exata da spec e Retry-After correto.
func (m *RateLimitMiddleware) respond429(w http.ResponseWriter, blockTime time.Duration) {
	message := "you have reached the maximum number of requests or actions allowed within a certain time frame"

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Retry-After", fmt.Sprintf("%d", int(blockTime.Seconds())))
	w.WriteHeader(http.StatusTooManyRequests)

	w.Write([]byte(message))
}
