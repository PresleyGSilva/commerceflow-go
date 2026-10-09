# CommerceFlow

Modular-monolith e-commerce demo: **Go (Chi + pgx)** API, **Next.js 15** storefront + admin, **Postgres 16**.
Money is always integer **cents** — never floats.

- Storefront (light, emerald `#059669` + amber `#F59E0B`): catalog, product pages, cart, coupons, addresses, idempotent checkout.
- Admin (slate): dashboard (revenue / orders / low-stock / top sellers), products, categories, stock movements + manual adjustments, orders + status, coupons, sales/stock reports. Admin-only guard (role `admin`, redirects to `/login`).
- Checkout: `POST /api/v1/checkout` with `Idempotency-Key` header, `SELECT … FOR UPDATE` on stock inside a transaction, server-side totals, coupon application, simulated payment (`approve` → `paid`, `decline`/`fail` → `failed` with stock restored).

Docs: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) · API spec: [`openapi.yaml`](openapi.yaml) (also served at `GET /openapi.yaml`) · [README em português](README.pt-BR.md).

## Stack

| Layer    | Tech |
|----------|------|
| API      | Go 1.23+, Chi, pgx/v5, JWT (access 15 min + refresh httpOnly cookie), bcrypt, slog JSON |
| DB       | Postgres 16, versioned `migrations/NNN_*.sql`, minimal ordered migrator |
| Web      | Next.js 15 + TypeScript + Tailwind, `npm run start` on port 3002 |
| Contracts| `{data,…}` responses, `{error:{code,message}}` errors, `X-Org-ID` / `X-User` tenant headers |

## Quickstart (docker compose)

```bash
cp .env.example .env   # optional; defaults already work
docker compose up --build
```

- Web: http://localhost:3002 (or `$WEB_PORT`)
- API: http://localhost:8082 (or `$API_PORT`) — `/healthz`, `/readyz`, `/openapi.yaml`
- DB: localhost:5435 (or `$DB_PORT`)

API base URL for the web build is baked via `ARG NEXT_PUBLIC_API_URL` (see `web/Dockerfile`, `docker-compose.yml`).

### Local dev without compose

```bash
# DB
docker run -d --name cf-db -e POSTGRES_USER=commerce -e POSTGRES_PASSWORD=commerce \
  -e POSTGRES_DB=commerceflow -p 5435:5432 postgres:16

# API
export DATABASE_URL=postgres://commerce:commerce@localhost:5435/commerceflow?sslmode=disable
export JWT_SECRET=change-me-in-production
go run ./cmd/api        # :8082, runs migrations + seed, graceful shutdown

# Web
cd web && npm install && npm run dev   # :3002 (NEXT_PUBLIC_API_URL=http://localhost:8082)
```

## Seed credentials (fictitious)

| Role     | Email                    | Password      |
|----------|--------------------------|---------------|
| admin    | admin@commerceflow.local | `Admin123!`   |
| customer | ana@exemplo.com          | `Customer123!`|

Seed also creates org `Loja Demo` (both users are members), categories (Eletrônicos, Moda, Casa, Esporte), ~12 products with cent prices, and coupons `BEMVINDO10` (percent 10) and `FRETE5` (fixed 500 cents).

Login returns `orgs[]`; the frontend persists `orgs[0].id` and sends it as `X-Org-ID` on every authenticated call.

## Main endpoints

| Method | Path | Notes |
|--------|------|-------|
| POST | `/api/v1/auth/register` | `{name,email,password}` → 201 `{data:{token,user,orgs}}` |
| POST | `/api/v1/auth/login` | → `{data:{token,user,orgs}}` + refresh cookie |
| POST | `/api/v1/auth/refresh` | httpOnly cookie → new access token |
| GET | `/api/v1/products?q=&category=&min_price=&max_price=&sort=&page=&per_page=` | public; prices in cents |
| GET | `/api/v1/products/{slug}` | public detail |
| GET | `/api/v1/categories` | public |
| GET | `/api/v1/coupons/validate?code=&subtotal=` | preview discount |
| GET/POST | `/api/v1/addresses` | auth |
| POST | `/api/v1/checkout` | auth + `Idempotency-Key` (see below) |
| GET | `/api/v1/orders`, `/api/v1/orders/{id}` | auth, own orders |
| GET | `/api/v1/admin/dashboard` | admin: revenue/orders/low-stock/top |
| GET | `/api/v1/admin/reports/sales?from=&to=&group_by=day` | admin |
| GET | `/api/v1/admin/reports/stock` | admin |
| CRUD | `/api/v1/admin/products`, `/api/v1/admin/categories`, `/api/v1/admin/coupons` | admin |
| PATCH | `/api/v1/admin/products/{id}/stock` `{delta,reason}` | admin stock adjust |
| GET | `/api/v1/admin/stock/movements` | admin |
| GET/PATCH | `/api/v1/admin/orders`, `/api/v1/admin/orders/{id}/status` | admin |

Order status: `pending|paid|failed|cancelled|shipped`. Payment modes: `approve|decline|fail` (simulated, no network).

### Idempotent checkout (curl)

```bash
TOKEN=$(curl -s -X POST localhost:8082/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ana@exemplo.com","password":"Customer123!"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])')

PID=$(curl -s localhost:8082/api/v1/products?per_page=1 | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"][0]["id"])')

KEY=$(python3 -c 'import uuid;print(uuid.uuid4())')
BODY="{\"items\":[{\"product_id\":\"$PID\",\"qty\":1}],\"coupon_code\":\"BEMVINDO10\",\"payment_mode\":\"approve\"}"

# First attempt → 201 created (paid)
curl -s -X POST localhost:8082/api/v1/checkout -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $KEY" -H 'Content-Type: application/json' -d "$BODY"

# Retry with the SAME key → 200 same order, no extra stock movement
curl -s -X POST localhost:8082/api/v1/checkout -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $KEY" -H 'Content-Type: application/json' -d "$BODY"
```

## Tests

```bash
go vet ./...
go test ./...            # unit: coupon math, slug, concurrency-safe discount calc
cd web && npx tsc --noEmit && npm run build
node tests/e2e.mjs       # real end-to-end purchase against local stack (no mocks)
```

The E2E script (`tests/e2e.mjs`) runs a full purchase — register → login → catalog → cart → `checkout(approve)` → order `paid` — against `http://localhost:3002` + API `:8082`, and saves real screenshots to `docs/screenshots/`. It needs the stack up (`docker compose up` or local API+web). If screenshots are missing, the reason is declared in `docs/screenshots/README.md`.

## Screenshots

| Page | File |
|------|------|
| Home | `docs/screenshots/home.png` |
| Catalog | `docs/screenshots/catalogo.png` |
| Product | `docs/screenshots/produto.png` |
| Checkout | `docs/screenshots/checkout.png` |
| Admin | `docs/screenshots/admin.png` |

## Architecture

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md): modules, checkout transaction, idempotency, payment simulator, RBAC/tenancy, admin frontend, E2E.

## License

MIT — see [LICENSE](LICENSE).
