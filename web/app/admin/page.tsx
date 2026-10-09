"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { DashboardData } from "@/lib/admin";
import { formatBRL } from "@/lib/money";

function Card({
  testid,
  label,
  value,
  sub,
}: {
  testid: string;
  label: string;
  value: string;
  sub?: string;
}) {
  return (
    <div
      data-testid={testid}
      className="rounded-xl border border-slate-800 bg-slate-900 p-5"
    >
      <p className="text-xs font-semibold uppercase tracking-widest text-slate-400">{label}</p>
      <p className="mt-2 text-3xl font-bold text-white">{value}</p>
      {sub && <p className="mt-1 text-xs text-slate-400">{sub}</p>}
    </div>
  );
}

export default function AdminDashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api<DashboardData>("/api/v1/admin/dashboard")
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : "Falha ao carregar"));
  }, []);

  if (error)
    return (
      <p role="alert" className="text-sm text-red-400" data-testid="admin-error">
        {error}
      </p>
    );
  if (!data) return <p className="text-sm text-slate-400">Carregando…</p>;

  return (
    <div data-testid="admin-dashboard">
      <h1 className="text-xl font-bold text-white">Dashboard</h1>
      <div className="mt-4 grid gap-4 sm:grid-cols-3">
        <Card
          testid="admin-card-revenue"
          label="Receita (paid)"
          value={formatBRL(data.revenue_cents)}
          sub={`${data.paid_count} pedidos pagos`}
        />
        <Card testid="admin-card-orders" label="Pedidos" value={String(data.orders_count)} />
        <Card
          testid="admin-card-low-stock"
          label="Baixo estoque (≤5)"
          value={String(data.low_stock.length)}
        />
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-2">
        <section
          data-testid="admin-low-stock"
          className="rounded-xl border border-slate-800 bg-slate-900 p-5"
        >
          <h2 className="font-semibold text-white">Baixo estoque</h2>
          {data.low_stock.length === 0 ? (
            <p className="mt-2 text-sm text-slate-400">Nenhum produto em baixo estoque.</p>
          ) : (
            <ul className="mt-3 space-y-2 text-sm">
              {data.low_stock.map((p) => (
                <li
                  key={p.id}
                  className="flex items-center justify-between gap-2 rounded-lg bg-slate-800/60 px-3 py-2"
                >
                  <span className="truncate text-slate-100">{p.name}</span>
                  <span className="shrink-0 rounded-full bg-amber-500/15 px-2 py-0.5 font-mono text-xs font-bold text-amber-300">
                    {p.stock_qty} un
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section
          data-testid="admin-top-products"
          className="rounded-xl border border-slate-800 bg-slate-900 p-5"
        >
          <h2 className="font-semibold text-white">Mais vendidos</h2>
          {data.top_products.length === 0 ? (
            <p className="mt-2 text-sm text-slate-400">Sem vendas pagas ainda.</p>
          ) : (
            <ul className="mt-3 space-y-2 text-sm">
              {data.top_products.map((p) => (
                <li
                  key={p.id}
                  className="flex items-center justify-between gap-2 rounded-lg bg-slate-800/60 px-3 py-2"
                >
                  <span className="truncate text-slate-100">
                    {p.name} <span className="text-slate-400">× {p.qty}</span>
                  </span>
                  <span className="shrink-0 font-mono text-xs font-bold text-emerald-300">
                    {formatBRL(p.revenue_cents)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}
