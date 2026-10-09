"use client";

import { useEffect, useState } from "react";
import { api, HttpError } from "@/lib/api";
import type { Product } from "@/lib/api";
import { unwrapList, type StockMovement } from "@/lib/admin";

const inputCls =
  "w-full rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white placeholder:text-slate-500";

export default function AdminEstoque() {
  const [movements, setMovements] = useState<StockMovement[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [error, setError] = useState("");
  const [formError, setFormError] = useState("");
  const [formOk, setFormOk] = useState("");
  const [productId, setProductId] = useState("");
  const [delta, setDelta] = useState("");
  const [reason, setReason] = useState("");

  async function reload() {
    try {
      const res = await api<StockMovement[] | { items: StockMovement[] }>(
        "/api/v1/admin/stock/movements?per_page=50",
      );
      setMovements(unwrapList(res));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao carregar movimentações");
    }
  }

  useEffect(() => {
    reload();
    api<Product[] | { items: Product[] }>("/api/v1/products?per_page=100", { auth: false })
      .then((res) => setProducts(unwrapList(res)))
      .catch(() => undefined);
  }, []);

  async function adjust(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    setFormOk("");
    const d = Number(delta);
    if (!productId || !Number.isInteger(d) || d === 0) {
      setFormError("Escolha o produto e informe um delta inteiro diferente de zero.");
      return;
    }
    try {
      const res = await api<{ stock_qty: number; product_id?: string; id?: string }>(
        `/api/v1/admin/products/${productId}/stock`,
        { method: "PATCH", body: { delta: d, reason: reason || "manual adjustment" } },
      );
      setFormOk(`Estoque atualizado: ${res.stock_qty} un.`);
      setDelta("");
      setReason("");
      await reload();
    } catch (err) {
      setFormError(err instanceof HttpError ? err.message : "Falha ao ajustar");
    }
  }

  return (
    <div data-testid="admin-stock">
      <h1 className="text-xl font-bold text-white">Estoque</h1>
      {error && (
        <p role="alert" className="mt-3 text-sm text-red-400">
          {error}
        </p>
      )}

      <form
        onSubmit={adjust}
        data-testid="admin-stock-form"
        className="mt-4 grid gap-3 rounded-xl border border-slate-800 bg-slate-900 p-5 sm:grid-cols-3"
      >
        <h2 className="font-semibold text-white sm:col-span-3">Ajuste manual</h2>
        <label className="text-sm text-slate-300">
          Produto
          <select
            value={productId}
            onChange={(e) => setProductId(e.target.value)}
            className={`mt-1 ${inputCls}`}
          >
            <option value="">Selecionar…</option>
            {products.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.stock_qty} un)
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm text-slate-300">
          Delta (+ entrada / − saída)
          <input
            value={delta}
            onChange={(e) => setDelta(e.target.value)}
            inputMode="numeric"
            placeholder="+10 ou -3"
            className={`mt-1 font-mono ${inputCls}`}
          />
        </label>
        <label className="text-sm text-slate-300">
          Motivo
          <input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="manual adjustment"
            className={`mt-1 ${inputCls}`}
          />
        </label>
        {formError && (
          <p role="alert" className="text-sm text-red-400 sm:col-span-3">
            {formError}
          </p>
        )}
        {formOk && <p className="text-sm text-emerald-300 sm:col-span-3">{formOk}</p>}
        <div className="sm:col-span-3">
          <button
            type="submit"
            className="rounded-lg bg-amber-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-amber-400"
          >
            Aplicar ajuste
          </button>
        </div>
      </form>

      <h2 className="mt-6 font-semibold text-white">Movimentações recentes</h2>
      <div className="mt-2 overflow-x-auto rounded-xl border border-slate-800">
        <table className="w-full min-w-[560px] text-left text-sm">
          <thead>
            <tr className="bg-slate-900 text-xs uppercase tracking-wide text-slate-400">
              <th className="px-4 py-3">Produto</th>
              <th className="px-4 py-3">Delta</th>
              <th className="px-4 py-3">Motivo</th>
              <th className="px-4 py-3">Quando</th>
            </tr>
          </thead>
          <tbody className="bg-slate-900/50">
            {movements.map((m) => (
              <tr key={m.id} className="border-t border-slate-800" data-testid="admin-stock-row">
                <td className="px-4 py-2 text-slate-100">{m.product_name}</td>
                <td
                  className={`px-4 py-2 font-mono font-bold ${m.delta >= 0 ? "text-emerald-300" : "text-red-300"}`}
                >
                  {m.delta >= 0 ? `+${m.delta}` : m.delta}
                </td>
                <td className="max-w-[220px] truncate px-4 py-2 font-mono text-xs text-slate-400">
                  {m.reason}
                </td>
                <td className="px-4 py-2 text-xs text-slate-400">
                  {new Date(m.created_at).toLocaleString("pt-BR")}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {movements.length === 0 && (
          <p className="bg-slate-900/50 px-4 py-6 text-sm text-slate-400">
            Nenhuma movimentação registrada.
          </p>
        )}
      </div>
    </div>
  );
}
