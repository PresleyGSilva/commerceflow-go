# CommerceFlow — Architecture

Modular Go monolith (Chi + pgx) with a Next.js storefront/admin web app
backed by Postgres 16.

## Modules

- `cmd/api`: entrypoint — loads config, connects pgxpool, runs SQL
  migrations in order, serves HTTP with graceful shutdown.
- `internal/config`: environment (`DATABASE_URL`, `JWT_SECRET`, `PORT`,
  `REFRESH_TTL_H`).
- `internal/db`: pgxpool connection + minimal ordered migrator over
  `migrations/*.sql` (tracked in `schema_migrations`).
- `internal/auth`: register/login/refresh/logout/me; bcrypt (cost 12);
  JWT access tokens (15 min, claims `sub,email,role,org_id`) + opaque
  refresh tokens in httpOnly cookies; `Auth` Bearer middleware and
  `RequireRole("admin")` guard for `/api/v1/admin/*`.
- `internal/catalog`: categories, products, coupons, cart math.
- `internal/orders`: checkout transaction, order history, admin order
  management.
- `internal/payment`: simulated gateway, no network.
- `internal/admin`: dashboard aggregates and sales/stock reports.
- `internal/http`: Chi router, `/healthz`, `/readyz`, `/openapi.yaml`,
  CORS, request-id, slog JSON logs, `{data,...}` / `{error:{code,message}}`
  envelopes.

## Money

All money is integer **cents** (`price_cents`, `subtotal_cents`,
`discount_cents`, `total_cents`). Never floats. Totals are computed
server-side; coupons apply in cents (`percent` rounds down,
`fixed` capped at the subtotal).

## Checkout transaction

`POST /api/v1/checkout` (requires `Idempotency-Key` header):

1. Lock rows: `SELECT ... FOR UPDATE` on the ordered products
   (`ORDER BY id` to avoid deadlocks).
2. Validate stock; compute totals in cents; apply coupon.
3. Decrement stock, insert `orders` (pending), `order_items` snapshots
   (`name`, `unit_price_cents`), `stock_movements` (`reason order:<id>`),
   and the idempotency record.
4. Simulate payment **inside the same transaction window**:
   `approve` → `paid`; `decline`/`fail` → logical rollback (stock
   restored with compensating movements) and order marked `failed`.

## Idempotency

`(user_id, idempotency_key)` is unique. Repeating a key returns the
original order (`200`) without touching stock or creating duplicates.

## Payment simulator

`internal/payment.Simulate(mode)`: `approve` → approved with auth code,
`decline` → declined (order `failed`, stock restored), `fail` → gateway
error (order `failed`, stock restored). Deterministic, no network.

## RBAC & tenancy

Roles: `admin` | `customer`. Login returns `orgs[]`; clients persist
`orgs[0].id` and send it as `X-Org-ID` (tenant scope); `X-User` is
optional (logging). All `/api/v1/admin/*` routes require the admin role
(401 without token, 403 for customers).

## Admin frontend (`web/app/admin/*`)

Slate shell (`bg-slate-950`, cards `bg-slate-900`) with emerald/amber
accents. `app/admin/layout.tsx` guards every admin route client-side
(non-admin → `/login`); the API enforces `RequireRole("admin")`
independently. Pages: `/admin` (dashboard cards + low-stock/top lists),
`/admin/produtos`, `/admin/categorias`, `/admin/estoque` (movements +
`PATCH …/stock {delta,reason}`), `/admin/pedidos` (status transitions),
`/admin/cupons`, `/admin/relatorios` (daily sales bars + valued stock).
The root `Header` renders a slate variant on `/admin*` and the light
store header everywhere else. All admin tables carry `data-testid="admin-*"`.

## E2E (`tests/e2e.mjs`)

Real purchase against a live stack (no mocks): register → login →
catalog → coupon preview → `checkout(approve)` → `paid` → idempotent
replay (same order id) → order in history → admin dashboard revenue.
Then Chromium screenshots to `docs/screenshots/` (home, catalogo,
produto, checkout with a seeded cart, admin, admin-produtos).
`API_URL`/`WEB_URL` envs override the default `localhost:8082/3002`
targets; `CHROME_PATH` overrides the browser binary.
