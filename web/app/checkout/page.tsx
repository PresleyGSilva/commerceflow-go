"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { API_URL, HttpError, ORG_KEY, TOKEN_KEY, USER_KEY, api } from "@/lib/api";
import type { Address, Order } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useCart } from "@/lib/cart";

function newIdempotencyKey(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

async function checkoutRequest(body: Record<string, unknown>): Promise<Order> {
  const headers = new Headers({ "Content-Type": "application/json" });
  // Fresh Idempotency-Key per attempt; repeat a key only to retry the same attempt.
  headers.set("Idempotency-Key", newIdempotencyKey());
  try {
    const token = window.localStorage.getItem(TOKEN_KEY);
    if (token) headers.set("Authorization", `Bearer ${token}`);
    const org = window.localStorage.getItem(ORG_KEY);
    if (org) headers.set("X-Org-ID", org);
    const raw = window.localStorage.getItem(USER_KEY);
    if (raw) {
      const parsed: unknown = JSON.parse(raw);
      if (parsed && typeof parsed === "object" && "id" in parsed && typeof parsed.id === "string")
        headers.set("X-User", parsed.id);
    }
  } catch {
    // storage unavailable: proceed without tenant headers
  }
  const res = await fetch(`${API_URL}/api/v1/checkout`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
    credentials: "include",
  });
  const json: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    if (json && typeof json === "object" && "error" in json) {
      const err: unknown = json.error;
      if (err && typeof err === "object" && "message" in err && typeof err.message === "string")
        message = err.message;
    }
    throw new HttpError(res.status, "checkout_failed", message);
  }
  if (json && typeof json === "object" && "data" in json) return json.data as Order;
  return json as Order;
}

export default function CheckoutPage() {
  const { user } = useAuth();
  const { lines, subtotalCents, clear } = useCart();
  const router = useRouter();
  const [addressId, setAddressId] = useState("");
  const [street, setStreet] = useState("");
  const [city, setCity] = useState("");
  const [state, setState] = useState("");
  const [zip, setZip] = useState("");
  const [coupon, setCoupon] = useState("");
  const [paymentMode, setPaymentMode] = useState<"approve" | "decline" | "fail">("approve");
  const [addresses, setAddresses] = useState<Address[]>([]);
  const [error, setError] = useState("");
  const [placing, setPlacing] = useState(false);
  const [loadedAddr, setLoadedAddr] = useState(false);

  async function loadAddresses() {
    try {
      const res = await api<Address[] | { items: Address[] }>("/api/v1/addresses");
      const list = Array.isArray(res) ? res : (res.items ?? []);
      setAddresses(list);
      if (list[0]) setAddressId(list[0].id);
    } catch {
      // guest or endpoint unavailable: manual address form stays
    }
  }
  if (user && !loadedAddr) {
    setLoadedAddr(true);
    void loadAddresses();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (lines.length === 0) {
      setError("Carrinho vazio.");
      return;
    }
    setPlacing(true);
    try {
      const body: Record<string, unknown> = {
        items: lines.map((l) => ({ product_id: l.product.id, qty: l.qty })),
        payment_mode: paymentMode,
      };
      if (coupon.trim()) body.coupon_code = coupon.trim().toUpperCase();
      if (addressId) body.address_id = addressId;
      else body.address = { street, city, state, zip, country: "BR" };
      const created = await checkoutRequest(body);
      clear();
      router.push(`/pedidos/${created.id}`);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Falha no checkout");
    } finally {
      setPlacing(false);
    }
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold">Checkout</h1>
      {!user && (
        <p className="mt-2 rounded-lg bg-amber-50 px-4 py-2 text-sm text-amber-800">
          Você está finalizando como visitante. <a href="/login" className="underline">Entre</a>{" "}
          para salvar endereços e ver pedidos.
        </p>
      )}
      <form onSubmit={submit} className="mt-6 space-y-5">
        <fieldset className="rounded-xl border border-stone-200 bg-white p-5">
          <legend className="px-2 text-sm font-bold">Endereço de entrega</legend>
          {addresses.length > 0 && (
            <div>
              <label htmlFor="addr-saved" className="text-sm font-medium">
                Endereço salvo
              </label>
              <select
                id="addr-saved"
                value={addressId}
                onChange={(e) => setAddressId(e.target.value)}
                className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
              >
                <option value="">— Novo endereço —</option>
                {addresses.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.street}, {a.city}/{a.state}
                  </option>
                ))}
              </select>
            </div>
          )}
          {!addressId && (
            <div className="mt-3 grid gap-3">
              <div>
                <label htmlFor="co-street" className="text-sm font-medium">
                  Rua e número
                </label>
                <input
                  id="co-street"
                  required
                  value={street}
                  onChange={(e) => setStreet(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
                />
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label htmlFor="co-city" className="text-sm font-medium">
                    Cidade
                  </label>
                  <input
                    id="co-city"
                    required
                    value={city}
                    onChange={(e) => setCity(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
                  />
                </div>
                <div>
                  <label htmlFor="co-state" className="text-sm font-medium">
                    UF
                  </label>
                  <input
                    id="co-state"
                    required
                    value={state}
                    onChange={(e) => setState(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
                  />
                </div>
                <div>
                  <label htmlFor="co-zip" className="text-sm font-medium">
                    CEP
                  </label>
                  <input
                    id="co-zip"
                    required
                    value={zip}
                    onChange={(e) => setZip(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
                  />
                </div>
              </div>
            </div>
          )}
        </fieldset>

        <fieldset className="rounded-xl border border-stone-200 bg-white p-5">
          <legend className="px-2 text-sm font-bold">Pagamento (simulado)</legend>
          <label htmlFor="co-coupon" className="text-sm font-medium">
            Cupom
          </label>
          <input
            id="co-coupon"
            value={coupon}
            onChange={(e) => setCoupon(e.target.value.toUpperCase())}
            placeholder="BEMVINDO10"
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2 uppercase"
          />
          <label htmlFor="co-paymode" className="mt-3 block text-sm font-medium">
            Modo de pagamento (demo)
          </label>
          <select
            id="co-paymode"
            value={paymentMode}
            onChange={(e) => setPaymentMode(e.target.value as "approve" | "decline" | "fail")}
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
          >
            <option value="approve">approve — aprova e marca paid</option>
            <option value="decline">decline — recusa (failed, estorna estoque)</option>
            <option value="fail">fail — erro gateway (failed, estorna estoque)</option>
          </select>
        </fieldset>

        <div className="rounded-xl border border-stone-200 bg-white p-5 text-sm">
          <p className="flex justify-between">
            <span>Subtotal ({lines.reduce((n, l) => n + l.qty, 0)} itens)</span>
            <strong>
              {(subtotalCents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })}
            </strong>
          </p>
        </div>

        {error && (
          <p className="text-sm text-red-600" role="alert">
            {error}
          </p>
        )}
        <button
          type="submit"
          data-testid="checkout-submit"
          disabled={placing || lines.length === 0}
          className="w-full rounded-lg bg-brand-600 px-4 py-3 font-semibold text-white hover:bg-brand-700 disabled:bg-stone-300"
        >
          {placing ? "Processando…" : "Finalizar pedido"}
        </button>
      </form>
    </div>
  );
}
