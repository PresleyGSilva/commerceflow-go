// CommerceFlow E2E: full purchase against a REAL local stack (no mocks).
//
// Needs the stack up (docker compose up, or local API + web):
//   API_URL default http://localhost:8082, WEB_URL default http://localhost:3002
// If host ports clash with unrelated containers, override:
//   API_URL=http://localhost:18082 WEB_URL=http://localhost:13002 node e2e.mjs
//
// Flow: register -> login -> catalog -> coupon preview -> checkout(approve)
//   -> order paid -> idempotent replay (same order) -> admin dashboard.
// Screenshots (real Chromium render): docs/screenshots/*.png
//
// Exit non-zero on the first failed assertion.

import { randomUUID } from "node:crypto";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const SHOTS = join(ROOT, "docs", "screenshots");
const API = (process.env.API_URL ?? "http://localhost:8082").replace(/\/$/, "");
const WEB = (process.env.WEB_URL ?? "http://localhost:3002").replace(/\/$/, "");

const results = [];
function check(name, cond, extra = "") {
  results.push({ name, ok: !!cond, extra });
  console.log(`${cond ? "PASS" : "FAIL"}  ${name}${extra ? ` — ${extra}` : ""}`);
  if (!cond) {
    console.error(`\nE2E FAILED at: ${name}${extra ? ` (${extra})` : ""}`);
    process.exit(1);
  }
}

async function apiJson(path, { method = "GET", token = null, orgId = null, key = null, body = undefined } = {}) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  if (orgId) headers["X-Org-ID"] = orgId;
  if (key) headers["Idempotency-Key"] = key;
  const res = await fetch(`${API}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const json = await res.json().catch(() => null);
  return { status: res.status, json };
}

function dataOf(res) {
  return res.json && typeof res.json === "object" && "data" in res.json
    ? res.json.data
    : res.json;
}

// ---------- 1. API alive ----------
{
  const res = await apiJson("/healthz");
  check("api healthz", res.status === 200, `HTTP ${res.status}`);
}

// ---------- 2. register + login (customer) ----------
const stamp = Date.now().toString(36);
const email = `e2e-${stamp}@exemplo.com`;
const password = "E2e12345!";
let reg = await apiJson("/api/v1/auth/register", {
  method: "POST",
  body: { name: "E2E Buyer", email, password },
});
check("register customer", reg.status === 201 || reg.status === 200, `HTTP ${reg.status}`);

const login = await apiJson("/api/v1/auth/login", {
  method: "POST",
  body: { email, password },
});
check("login customer", login.status === 200, `HTTP ${login.status}`);
const session = dataOf(login);
const token = session.access_token ?? session.token;
const orgs = session.orgs ?? session.user?.orgs ?? [];
check("login returns token + orgs", !!token && orgs.length > 0, `orgs=${orgs.length}`);
const orgId = orgs[0].id;
const userId = session.user?.id ?? null;

// ---------- 3. catalog ----------
const prods = await apiJson("/api/v1/products?per_page=12");
check("list products", prods.status === 200, `HTTP ${prods.status}`);
const products = dataOf(prods);
const list = Array.isArray(products) ? products : (products.items ?? []);
check("catalog non-empty", list.length > 0, `${list.length} products`);
const chosen = list.find((p) => p.stock_qty > 1) ?? list[0];
check("product with stock", chosen.stock_qty > 0, `${chosen.slug} stock=${chosen.stock_qty}`);

// ---------- 4. coupon preview ----------
const preview = await apiJson(
  `/api/v1/coupons/validate?code=BEMVINDO10&subtotal=${chosen.price_cents}`,
);
check("coupon BEMVINDO10 valid", preview.status === 200, `HTTP ${preview.status}`);

// ---------- 5. checkout approve ----------
const idemKey = randomUUID();
const qty = 1;
const orderBody = {
  items: [{ product_id: chosen.id, qty }],
  coupon_code: "BEMVINDO10",
  payment_mode: "approve",
};
const co = await apiJson("/api/v1/checkout", {
  method: "POST",
  token,
  orgId,
  key: idemKey,
  body: orderBody,
});
check("checkout approve 201", co.status === 201 || co.status === 200, `HTTP ${co.status}`);
const order = dataOf(co);
check("order paid", order.status === "paid", `status=${order.status}`);
check(
  "totals consistent",
  order.subtotal_cents - order.discount_cents === order.total_cents && order.discount_cents > 0,
  `sub=${order.subtotal_cents} disc=${order.discount_cents} total=${order.total_cents}`,
);

// ---------- 6. idempotent replay ----------
const replay = await apiJson("/api/v1/checkout", {
  method: "POST",
  token,
  orgId,
  key: idemKey,
  body: orderBody,
});
check("idempotent replay same order", replay.status === 200 && dataOf(replay).id === order.id,
  `HTTP ${replay.status}`);

// ---------- 7. order visible in history ----------
const mine = await apiJson("/api/v1/orders", { token, orgId });
const myOrders = (() => {
  const d = dataOf(mine);
  return Array.isArray(d) ? d : (d.items ?? []);
})();
check("order in history", myOrders.some((o) => o.id === order.id), `${myOrders.length} orders`);

// ---------- 8. admin dashboard reflects the sale ----------
const adminLogin = await apiJson("/api/v1/auth/login", {
  method: "POST",
  body: { email: "admin@commerceflow.local", password: "Admin123!" },
});
check("login admin", adminLogin.status === 200, `HTTP ${adminLogin.status}`);
const adminSession = dataOf(adminLogin);
const adminToken = adminSession.access_token ?? adminSession.token;
const adminOrgs = adminSession.orgs ?? adminSession.user?.orgs ?? [];
check("admin role", adminSession.user?.role === "admin", `role=${adminSession.user?.role}`);
const dash = await apiJson("/api/v1/admin/dashboard", {
  token: adminToken,
  orgId: adminOrgs[0]?.id ?? orgId,
});
check("admin dashboard", dash.status === 200 && dataOf(dash).revenue_cents > 0,
  `HTTP ${dash.status}`);

// ---------- 9. real browser screenshots ----------
mkdirSync(SHOTS, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROME_PATH ?? "/usr/bin/google-chrome",
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
try {
  const ctx = await browser.newContext({ viewport: { width: 1360, height: 900 } });

  async function shoot(name, path, setup) {
    const page = await ctx.newPage();
    await page.goto(`${WEB}/`, { waitUntil: "domcontentloaded" });
    await page.evaluate(
      ({ t, o, u }) => {
        if (t) window.localStorage.setItem("commerceflow_token", t);
        if (o) window.localStorage.setItem("commerceflow_org_id", o);
        if (u) window.localStorage.setItem("commerceflow_user", u);
      },
      { t: setup?.token ?? null, o: setup?.orgId ?? null, u: setup?.user ?? null },
    );
    if (setup?.cart) {
      await page.evaluate(
        (cart) => window.localStorage.setItem("commerceflow_cart", JSON.stringify(cart)),
        setup.cart,
      );
    }
    await page.goto(`${WEB}${path}`, { waitUntil: "networkidle", timeout: 30000 });
    await page.waitForTimeout(1200);
    await page.screenshot({ path: join(SHOTS, name) });
    console.log(`SHOT  ${name}`);
    await page.close();
  }

  const buyerAuth = {
    token,
    orgId,
    user: JSON.stringify({ id: userId, email, name: "E2E Buyer", role: "customer" }),
  };
  const adminAuth = {
    token: adminToken,
    orgId: adminOrgs[0]?.id ?? orgId,
    user: JSON.stringify({
      id: adminSession.user?.id,
      email: "admin@commerceflow.local",
      name: "Admin",
      role: "admin",
    }),
  };

  await shoot("home.png", "/");
  await shoot("catalogo.png", "/catalogo");
  await shoot("produto.png", `/produto/${chosen.slug}`);
  await shoot("checkout.png", "/checkout", {
    ...buyerAuth,
    cart: [{ product: chosen, qty: 1 }],
  });
  await shoot("admin.png", "/admin", adminAuth);
  await shoot("admin-produtos.png", "/admin/produtos", adminAuth);
} finally {
  await browser.close();
}

console.log(`\nE2E OK: ${results.filter((r) => r.ok).length}/${results.length} checks passed.`);
console.log(`API=${API} WEB=${WEB} order=${order.id} total_cents=${order.total_cents}`);
