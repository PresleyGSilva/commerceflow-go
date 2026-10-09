"use client";

import { useEffect, useState } from "react";
import { api, HttpError } from "@/lib/api";
import { unwrapList, type AdminOrder } from "@/lib/admin";
import { formatBRL } from "@/lib/money";

const STATUSES = ["pending", "paid", "failed", "cancelled", "shipped"];

function badge(status: string): string {
  if (status === "paid" || status === "shipped") return "bg-emerald-500/15 text-emerald-300";
  if (status === "pending") return "bg-amber-500/15 text-amber-300";
  return "bg-red-500/15 text-red-300";
}

export default function AdminPedidos() {
  const [orders, setOrders] = useState<AdminOrder[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [updating, setUpdating] = useState<string | null>(null);

  async function reload() {
    try {
      const res = await api<AdminOrder[] | { items: AdminOrder[] }>("/api/v1/admin/orders");
      setOrders(unwrapList(res));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao carregar pedidos");
    }
  }

  useEffect(() => {
    reload();
  }, []);

  async function setStatus(order: AdminOrder, status: string) {
    setUpdating(order.id);
    setNotice("");
    try {
      await api(`/api/v1/admin/orders/${order.id}/status`, {
        method: "PATCH",
        body: { status },
      });
      setNotice(`Pedido ${order.id.slice(0, 8)}… → ${status}`);
      await reload();
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Falha ao atualizar status");
    } finally {
      setUpdating(null);
    }
  }

  return (
    <div data-testid="admin-orders">
      <h1 className="text-xl font-bold text-white">Pedidos</h1>
      {error && (
        <p role="alert" className="mt-3 text-sm text-red-400">
          {error}
        </p>
      )}
      {notice && <p className="mt-3 text-sm text-emerald-300">{notice}</p>}

      <ul className="mt-4 space-y-3">
        {orders.map((o) => (
          <li
            key={o.id}
            data-testid="admin-order-row"
            className="rounded-xl border border-slate-800 bg-slate-900 p-4"
          >
            <div className="flex flex-wrap items-center gap-3">
              <span className="font-mono text-xs text-slate-400">{o.id.slice(0, 8)}…</span>
              <span
                className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${badge(o.status)}`}
              >
                {o.status}
              </span>
              <span className="ml-auto font-mono text-sm font-bold text-white">
                {formatBRL(o.total_cents)}
              </span>
            </div>
            <p className="mt-1 text-xs text-slate-400">
              {new Date(o.created_at).toLocaleString("pt-BR")}
              {o.coupon_code ? ` · cupom ${o.coupon_code}` : ""}
              {typeof o.discount_cents === "number" && o.discount_cents > 0
                ? ` · desconto ${formatBRL(o.discount_cents)}`
                : ""}
            </p>
            {o.items && o.items.length > 0 && (
              <ul className="mt-2 space-y-1 text-xs text-slate-300">
                {o.items.map((it, i) => (
                  <li key={`${it.product_id}-${i}`}>
                    {it.product_name} × {it.qty} — {formatBRL(it.unit_price_cents)}
                  </li>
                ))}
              </ul>
            )}
            <div className="mt-3 flex flex-wrap gap-1.5">
              {STATUSES.filter((s) => s !== o.status).map((s) => (
                <button
                  key={s}
                  disabled={updating === o.id}
                  onClick={() => setStatus(o, s)}
                  className="rounded bg-slate-800 px-2.5 py-1 text-xs text-slate-200 hover:bg-slate-700 disabled:opacity-50"
                >
                  → {s}
                </button>
              ))}
            </div>
          </li>
        ))}
      </ul>
      {orders.length === 0 && !error && (
        <p className="mt-6 text-sm text-slate-400">Nenhum pedido ainda.</p>
      )}
    </div>
  );
}
