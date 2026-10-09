# CommerceFlow

Demo de e-commerce em monólito modular: API **Go (Chi + pgx)**, loja + admin em **Next.js 15**, **Postgres 16**.
Dinheiro sempre em **centavos (inteiro)** — nunca float.

- Loja (clara, esmeralda `#059669` + âmbar `#F59E0B`): catálogo, páginas de produto, carrinho, cupons, endereços, checkout idempotente.
- Admin (slate): dashboard (receita / pedidos / baixo estoque / mais vendidos), produtos, categorias, movimentações de estoque + ajuste manual, pedidos + status, cupons, relatórios de vendas/estoque. Guarda de rota: só `admin` (redireciona para `/login`).
- Checkout: `POST /api/v1/checkout` com header `Idempotency-Key`, `SELECT … FOR UPDATE` no estoque dentro de transação, totais calculados no servidor, pagamento simulado (`approve` → `paid`, `decline`/`fail` → `failed` com estorno do estoque).

Docs: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) · Spec da API: [`openapi.yaml`](openapi.yaml) (também servida em `GET /openapi.yaml`) · [README in English](README.md).

## Stack

| Camada   | Tecnologia |
|----------|------------|
| API      | Go 1.23+, Chi, pgx/v5, JWT (access 15 min + refresh em cookie httpOnly), bcrypt, slog JSON |
| Banco    | Postgres 16, `migrations/NNN_*.sql` versionadas, migrador próprio ordenado |
| Web      | Next.js 15 + TypeScript + Tailwind, `npm run start` na porta 3002 |
| Contratos| Respostas `{data,…}`, erros `{error:{code,message}}`, headers de tenant `X-Org-ID` / `X-User` |

## Quickstart (docker compose)

```bash
cp .env.example .env   # opcional; os defaults já funcionam
docker compose up --build
```

- Loja: http://localhost:3002 (ou `$WEB_PORT`)
- API: http://localhost:8082 (ou `$API_PORT`) — `/healthz`, `/readyz`, `/openapi.yaml`
- Banco: localhost:5435 (ou `$DB_PORT`)

A URL da API no build web é fixada via `ARG NEXT_PUBLIC_API_URL` (ver `web/Dockerfile` e `docker-compose.yml`).

### Dev local sem compose

```bash
# Banco
docker run -d --name cf-db -e POSTGRES_USER=commerce -e POSTGRES_PASSWORD=commerce \
  -e POSTGRES_DB=commerceflow -p 5435:5432 postgres:16

# API
export DATABASE_URL=postgres://commerce:commerce@localhost:5435/commerceflow?sslmode=disable
export JWT_SECRET=change-me-in-production
go run ./cmd/api        # :8082, roda migrations + seed, graceful shutdown

# Web
cd web && npm install && npm run dev   # :3002 (NEXT_PUBLIC_API_URL=http://localhost:8082)
```

## Credenciais do seed (fictícias)

| Perfil   | E-mail                   | Senha         |
|----------|--------------------------|---------------|
| admin    | admin@commerceflow.local | `Admin123!`   |
| customer | ana@exemplo.com          | `Customer123!`|

O seed também cria a org `Loja Demo` (ambos os usuários são membros), categorias (Eletrônicos, Moda, Casa, Esporte), ~12 produtos com preços em centavos e os cupons `BEMVINDO10` (percent 10) e `FRETE5` (fixed 500 centavos).

O login retorna `orgs[]`; o frontend persiste `orgs[0].id` e o envia como `X-Org-ID` em toda chamada autenticada.

## Principais endpoints

| Método | Rota | Notas |
|--------|------|-------|
| POST | `/api/v1/auth/register` | `{name,email,password}` → 201 `{data:{token,user,orgs}}` |
| POST | `/api/v1/auth/login` | → `{data:{token,user,orgs}}` + cookie de refresh |
| POST | `/api/v1/auth/refresh` | cookie httpOnly → novo access token |
| GET | `/api/v1/products?q=&category=&min_price=&max_price=&sort=&page=&per_page=` | público; preços em centavos |
| GET | `/api/v1/products/{slug}` | detalhe público |
| GET | `/api/v1/categories` | público |
| GET | `/api/v1/coupons/validate?code=&subtotal=` | prévia do desconto |
| GET/POST | `/api/v1/addresses` | autenticado |
| POST | `/api/v1/checkout` | autenticado + `Idempotency-Key` (ver abaixo) |
| GET | `/api/v1/orders`, `/api/v1/orders/{id}` | autenticado, próprios pedidos |
| GET | `/api/v1/admin/dashboard` | admin: receita/pedidos/baixo-estoque/top |
| GET | `/api/v1/admin/reports/sales?from=&to=&group_by=day` | admin |
| GET | `/api/v1/admin/reports/stock` | admin |
| CRUD | `/api/v1/admin/products`, `/api/v1/admin/categories`, `/api/v1/admin/coupons` | admin |
| PATCH | `/api/v1/admin/products/{id}/stock` `{delta,reason}` | ajuste de estoque (admin) |
| GET | `/api/v1/admin/stock/movements` | admin |
| GET/PATCH | `/api/v1/admin/orders`, `/api/v1/admin/orders/{id}/status` | admin |

Status do pedido: `pending|paid|failed|cancelled|shipped`. Modos de pagamento: `approve|decline|fail` (simulado, sem rede).

### Checkout idempotente (curl)

```bash
TOKEN=$(curl -s -X POST localhost:8082/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ana@exemplo.com","password":"Customer123!"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])')

PID=$(curl -s localhost:8082/api/v1/products?per_page=1 | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"][0]["id"])')

KEY=$(python3 -c 'import uuid;print(uuid.uuid4())')
BODY="{\"items\":[{\"product_id\":\"$PID\",\"qty\":1}],\"coupon_code\":\"BEMVINDO10\",\"payment_mode\":\"approve\"}"

# Primeira tentativa → 201 created (paid)
curl -s -X POST localhost:8082/api/v1/checkout -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $KEY" -H 'Content-Type: application/json' -d "$BODY"

# Repetindo a MESMA key → 200 mesmo pedido, sem nova movimentação de estoque
curl -s -X POST localhost:8082/api/v1/checkout -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $KEY" -H 'Content-Type: application/json' -d "$BODY"
```

## Testes

```bash
go vet ./...
go test ./...            # unitários: matemática de cupom, slug, cálculo de desconto
cd web && npx tsc --noEmit && npm run build
node tests/e2e.mjs       # compra real de ponta a ponta contra o stack local (sem mocks)
```

O script E2E (`tests/e2e.mjs`) faz a compra completa — registro → login → catálogo → carrinho → `checkout(approve)` → pedido `paid` — contra `http://localhost:3002` + API `:8082`, e salva screenshots reais em `docs/screenshots/`. Precisa do stack no ar (`docker compose up` ou API+web locais). Se os screenshots estiverem ausentes, o motivo está declarado em `docs/screenshots/README.md`.

## Screenshots

| Página | Arquivo |
|--------|---------|
| Home | `docs/screenshots/home.png` |
| Catálogo | `docs/screenshots/catalogo.png` |
| Produto | `docs/screenshots/produto.png` |
| Checkout | `docs/screenshots/checkout.png` |
| Admin | `docs/screenshots/admin.png` |

## Arquitetura

Ver [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md): módulos, transação do checkout, idempotência, simulador de pagamento, RBAC/tenancy, frontend admin, E2E.

## Licença

MIT — ver [LICENSE](LICENSE).
