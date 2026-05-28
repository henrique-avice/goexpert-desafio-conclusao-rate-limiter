# goexpert-desafio-conclusao-rate-limiter

> Rate limiter configurável por IP e token com backend Redis, padrão Strategy e fallback automático para memória.

## Índice

- [Visão Geral](#visão-geral)
- [Funcionalidades](#funcionalidades)
- [Requisitos](#requisitos)
- [Configuração](#configuração)
- [Execução](#execução)
- [Arquitetura](#arquitetura)
- [Testes](#testes)
- [Como Utilizar](#como-utilizar)

## Visão Geral

Middleware HTTP que limita requisições por **IP** e por **token** (header `API_KEY`). Implementa o padrão Strategy permitindo trocar o backend de armazenamento. Usa Redis como backend principal com fallback automático para in-memory quando o Redis fica indisponível.

## Funcionalidades

### Requisitos do Desafio

- [x] Limitação por endereço IP
- [x] Limitação por token via header `API_KEY` (token tem precedência sobre IP)
- [x] Resposta HTTP 429 com a mensagem exata: `you have reached the maximum number of requests or actions allowed within a certain time frame`
- [x] Backend Redis obrigatório
- [x] Padrão Strategy para intercambialidade do backend de armazenamento
- [x] Configuração via variáveis de ambiente (número de requisições por segundo, tempo de bloqueio)

### Extras Implementados

- Fail-open: após 3 falhas consecutivas do Redis, o limiter migra automaticamente para estratégia in-memory sem derrubar o serviço
- Recuperação automática: assim que o Redis volta a responder, o limiter retorna ao backend principal sem necessidade de reinício
- Header `Retry-After` na resposta 429 indicando quando o cliente pode tentar novamente

## Requisitos

- Go 1.26.2+
- Docker e Docker Compose
- Redis (provisionado pelo Docker Compose)

## Configuração

Copie o arquivo de exemplo e ajuste conforme necessário:

```bash
cp .env.example .env
```

| Variável | Padrão | Descrição |
|---|---|---|
| `RATE_LIMIT_IP_REQUESTS` | `10` | Requisições por segundo por IP |
| `RATE_LIMIT_TOKEN_REQUESTS` | `100` | Requisições por segundo por token |
| `RATE_LIMIT_BLOCK_TIME` | `300` | Tempo de bloqueio em segundos |
| `RATE_LIMIT_BACKEND` | `redis` | Backend: `redis` ou `memory` |
| `REDIS_HOST` | `localhost` | Host do Redis |
| `REDIS_PORT` | `6379` | Porta do Redis |
| `REDIS_DB` | `0` | Banco do Redis |
| `REDIS_PASSWORD` | `` | Senha do Redis |
| `SERVER_PORT` | `8080` | Porta do servidor HTTP |

## Execução

### Docker Compose (Recomendado)

```bash
docker-compose up --build
```

Sobe o Redis (`redis:7-alpine`) e a aplicação. A aplicação só inicia após o Redis estar saudável.

### Docker

```bash
docker build -t rate-limiter .
docker run -p 8080:8080 --env-file .env rate-limiter
```

Requer um Redis externo configurado nas variáveis de ambiente.

### Local

```bash
go run ./cmd/ratelimiter
```

## Arquitetura

```
[Cliente] ──► [chi Router :8080]
                    │
             [Middleware RateLimit]
                    │
              [RateLimiter]
               ┌────┴────┐
           [Redis]   [Memory]  ← fallback automático
                    │
             [Route Handler]
```

| Componente | Responsabilidade |
|---|---|
| `cmd/ratelimiter` | Inicialização do servidor e roteador chi |
| `internal/limiter` | Orquestração do rate limit e lógica de fallback |
| `internal/strategy` | Interface Strategy e implementações Redis e Memory |
| `internal/middleware` | Extração de IP/token e aplicação do middleware HTTP |
| `internal/config` | Leitura de variáveis de ambiente |

## Testes

```bash
go test -v -race ./...
```

---

## Como Utilizar

### 1. Iniciando o Sistema

```bash
docker-compose up --build
```

Sobe o Redis e o servidor em `:8080`. Aguardar a mensagem `Rate Limiter server starting` no log.

### 2. Testando

**Chamada normal (retorna 200):**

```bash
curl -i http://localhost:8080/api/test
```

**Testando com token (limite de 100 req/s em vez de 10):**

```bash
curl -i -H "API_KEY: meu-token-teste" http://localhost:8080/api/test
```

**Disparando o rate limit por IP (mais de 10 requisições rapidamente):**

```bash
# Bash / Linux / macOS
for i in $(seq 1 15); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/test; done

# PowerShell
1..15 | ForEach-Object { try { (Invoke-WebRequest -Uri "http://localhost:8080/api/test" -UseBasicParsing).StatusCode } catch { $_.Exception.Response.StatusCode.value__ } }
```

**Outros endpoints disponíveis:**

```bash
curl -i http://localhost:8080/api/users
curl -i -X POST http://localhost:8080/api/submit -H "Content-Type: application/json" -d '{"data":"teste"}'
curl -i http://localhost:8080/health
```

### 3. Resultado Esperado

```
# Requisições dentro do limite → HTTP 200
HTTP/1.1 200 OK
{"data":"test response","status":"ok"}

# GET /api/users
{"users":[{"id":"1","name":"Alice"},{"id":"2","name":"Bob"}],"status":"ok"}

# Ao ultrapassar 10 req/s por IP → HTTP 429
HTTP/1.1 429 Too Many Requests
Retry-After: 300

you have reached the maximum number of requests or actions allowed within a certain time frame
```

Após o bloqueio, o IP fica bloqueado por `RATE_LIMIT_BLOCK_TIME` segundos (padrão: 300s). Com o header `API_KEY`, o limite sobe para 100 req/s e o token é bloqueado independentemente do IP.
