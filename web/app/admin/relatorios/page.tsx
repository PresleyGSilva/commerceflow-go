"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { SalesReport, StockItem } from "@/lib/admin";
import { formatBRL } from "@/lib/money";

function isoDaysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
}

export default function AdminRelatorios() {
  const [from, setFrom] = useState(() => isoDaysAgo(29));
  const [to, setTo] = useState(() => isoDaysAgo(0));
  const [sales, setSales] = useState<SalesReport | null>(null);
  const [stock, setStock] = useState<StockItem[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    setError("");
    api<SalesReport>(`/api/v1/admin/reports/sales?from=${from}&to=${to}&group_by=day`)
      .then(setSales)
      .catch((e) => setError(e instanceof Error ? e.message : "Falha no relatório de vendas"));
    api<StockItem[] | { items: StockItem[] }>("/api/v1/admin/reports/stock")
      .then((res) => setStock(Array.isArray(res) ? res : (res.items ?? [])))
      .catch((e) => setError(e instanceof Error ? e.message : "Falha no relatório de estoque"));
  }, [from, to]);

  const maxRevenue = useMemo(
    () => sales?.points.reduce((m, p) => Math.max(m, p.revenue_cents), 0) ?? 0,
    [sales],
  );
  const totalRevenue = useMemo(
    () => sales?.points.reduce((n, p) => n + p.revenue_cents, 0) ?? 0,
    [sales],
  );
  const totalOrders = useMemo(
    () => sales?.points.reduce((n, p) => n + p.orders_count, 0) ?? 0,
    [sales],
  );
  const stockValue = useMemo(
    () => stock.reduce((n, s) => n + s.stock_value_cents, 0),
    [stock],
  );

  return (
    <div data-testid="admin-reports">
      <h1 className="text-xl font-bold text-white">Relatórios</h1>
      {error && (
        <p role="alert" className="mt-3 text-sm text-red-400">
          {error}
        </p>
      )}

      <section
        data-testid="admin-report-sales"
        className="mt-4 rounded-xl border border-slate-800 bg-slate-900 p-5"
      >
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="font-semibold text-white">Vendas por dia</h2>
          <div className="ml-auto flex items-center gap-2 text-xs text-slate-300">
            <label>
              De{" "}
              <input
                type="date"
                value={from}
                onChange={(e) => setFrom(e.target.value)}
                className="rounded bg-slate-800 px-2 py-1 text-slate-100"
              />
            </label>
            <label>
              Até{" "}
              <input
                type="date"
                value={to}
                onChange={(e) => setTo(e.target.value)}
                className="rounded bg-slate-800 px-2 py-1 text-slate-100"
              />
            </label>
          </div>
        </div>
        <p className="mt-2 text-sm text-slate-300">
          {totalOrders} pedidos pagos · {formatBRL(totalRevenue)} no período
        </p>
        {!sales ? (
          <p className="mt-4 text-sm text-slate-400">Carregando…</p>
        ) : (
          <div
            className="mt-4 flex h-40 items-end gap-1"
            role="img"
            aria-label={`Vendas diárias, total ${formatBRL(totalRevenue)}`}
          >
            {sales.points.map((p) => (
              <div
                key={p.date}
                title={`${p.date}: ${p.orders_count} pedidos · ${formatBRL(p.revenue_cents)}`}
                className="min-w-0 flex-1 rounded-t bg-emerald-500/80 hover:bg-emerald-400"
                style={{
                  height: `${maxRevenue > 0 ? Math.max(4, (p.revenue_cents / maxRevenue) * 100) : 4}%`,
                }}
              />
            ))}
          </div>
        )}
        {sales && sales.points.length > 0 && (
          <p className="mt-2 font-mono text-[11px] text-slate-500">
            {sales.points[0].date} → {sales.points[sales.points.length - 1].date}
          </p>
        )}
      </section>

      <section
        data-testid="admin-report-stock"
        className="mt-4 rounded-xl border border-slate-800 bg-slate-900 p-5"
      >
        <h2 className="font-semibold text-white">Estoque valorizado</h2>
        <p className="mt-1 text-sm text-slate-300">
          {stock.length} produtos · valor total {formatBRL(stockValue)}
        </p>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[520px] text-left text-sm">
            <thead>
              <tr className="text-xs uppercase tracking-wide text-slate-400">
                <th className="py-2 pr-4">Produto</th>
                <th className="py-2 pr-4">Qtd</th>
                <th className="py-2 pr-4">Preço</th>
                <th className="py-2">Valor em estoque</th>
              </tr>
            </thead>
            <tbody>
              {stock.map((s) => (
                <tr key={s.id} className="border-t border-slate-800">
                  <td className="py-2 pr-4 text-slate-100">
                    {s.name}{" "}
                    {s.low_stock && (
                      <span className="ml-1 rounded-full bg-amber-500/15 px-2 py-0.5 text-[11px] font-bold text-amber-300">
                        baixo
                      </span>
                    )}
                  </td>
                  <td className="py-2 pr-4 font-mono text-slate-200">{s.stock_qty}</td>
                  <td className="py-2 pr-4 font-mono text-slate-200">
                    {formatBRL(s.price_cents)}
                  </td>
                  <td className="py-2 font-mono text-slate-200">
                    {formatBRL(s.stock_value_cents)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
