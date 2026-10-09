"use client";

import { useEffect, useState } from "react";
import { api, HttpError } from "@/lib/api";
import { unwrapList, type Coupon } from "@/lib/admin";
import { formatBRL } from "@/lib/money";

const inputCls =
  "w-full rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white placeholder:text-slate-500";

type Form = {
  code: string;
  type: string;
  value: string;
  min_subtotal: string;
  active: boolean;
};

const emptyForm: Form = { code: "", type: "percent", value: "", min_subtotal: "", active: true };

function describe(c: Coupon): string {
  const amount =
    c.type === "percent" ? `${c.value}%` : formatBRL(c.value);
  const min =
    c.min_subtotal_cents > 0 ? ` · mín. ${formatBRL(c.min_subtotal_cents)}` : "";
  return `${amount}${min}`;
}

export default function AdminCupons() {
  const [coupons, setCoupons] = useState<Coupon[]>([]);
  const [error, setError] = useState("");
  const [formError, setFormError] = useState("");
  const [editing, setEditing] = useState<Coupon | null>(null);
  const [form, setForm] = useState<Form>(emptyForm);
  const [saving, setSaving] = useState(false);

  async function reload() {
    try {
      const res = await api<Coupon[] | { items: Coupon[] }>("/api/v1/admin/coupons");
      setCoupons(unwrapList(res));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao carregar cupons");
    }
  }

  useEffect(() => {
    reload();
  }, []);

  function openNew() {
    setEditing(null);
    setForm(emptyForm);
    setFormError("");
  }

  function openEdit(c: Coupon) {
    setEditing(c);
    setForm({
      code: c.code,
      type: c.type,
      value: String(c.value),
      min_subtotal: (c.min_subtotal_cents / 100).toFixed(2),
      active: c.active,
    });
    setFormError("");
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setFormError("");
    // Known fields only (backend DisallowUnknownFields on create).
    const value = Number(form.value);
    const body: Record<string, unknown> = {
      code: form.code.trim().toUpperCase(),
      type: form.type,
      value: form.type === "percent" ? value : Math.round(value * 100),
      min_subtotal_cents: Math.round(Number(form.min_subtotal || "0") * 100),
      active: form.active,
    };
    try {
      if (editing) {
        await api(`/api/v1/admin/coupons/${editing.id}`, { method: "PUT", body });
      } else {
        await api("/api/v1/admin/coupons", { method: "POST", body });
      }
      openNew();
      await reload();
    } catch (err) {
      setFormError(err instanceof HttpError ? err.message : "Falha ao salvar");
    } finally {
      setSaving(false);
    }
  }

  async function remove(c: Coupon) {
    if (!window.confirm(`Excluir cupom "${c.code}"?`)) return;
    try {
      await api(`/api/v1/admin/coupons/${c.id}`, { method: "DELETE" });
      await reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao excluir");
    }
  }

  return (
    <div data-testid="admin-coupons">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-white">Cupons</h1>
        <button
          data-testid="admin-coupon-new"
          onClick={openNew}
          className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-500"
        >
          Novo cupom
        </button>
      </div>
      {error && (
        <p role="alert" className="mt-3 text-sm text-red-400">
          {error}
        </p>
      )}

      <ul className="mt-4 space-y-2">
        {coupons.map((c) => (
          <li
            key={c.id}
            data-testid="admin-coupon-row"
            className="flex items-center gap-3 rounded-xl border border-slate-800 bg-slate-900 px-4 py-3"
          >
            <div className="min-w-0 flex-1">
              <p className="font-mono font-bold text-amber-300">{c.code}</p>
              <p className="text-xs text-slate-400">
                {describe(c)} {c.active ? "· ativo" : "· inativo"}
              </p>
            </div>
            <button
              onClick={() => openEdit(c)}
              className="shrink-0 rounded bg-slate-800 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-700"
            >
              Editar
            </button>
            <button
              onClick={() => remove(c)}
              className="shrink-0 rounded bg-red-950 px-3 py-1.5 text-xs text-red-300 hover:bg-red-900"
            >
              Excluir
            </button>
          </li>
        ))}
      </ul>
      {coupons.length === 0 && !error && (
        <p className="mt-4 text-sm text-slate-400">Nenhum cupom cadastrado.</p>
      )}

      <form
        onSubmit={submit}
        data-testid="admin-coupon-form"
        className="mt-6 grid gap-3 rounded-xl border border-slate-800 bg-slate-900 p-5 sm:grid-cols-2"
      >
        <h2 className="font-semibold text-white sm:col-span-2">
          {editing ? `Editar: ${editing.code}` : "Novo cupom"}
        </h2>
        <label className="text-sm text-slate-300">
          Código*
          <input
            value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value })}
            required
            placeholder="BEMVINDO10"
            className={`mt-1 font-mono ${inputCls}`}
          />
        </label>
        <label className="text-sm text-slate-300">
          Tipo*
          <select
            value={form.type}
            onChange={(e) => setForm({ ...form, type: e.target.value })}
            className={`mt-1 ${inputCls}`}
          >
            <option value="percent">percent (%)</option>
            <option value="fixed">fixed (R$)</option>
          </select>
        </label>
        <label className="text-sm text-slate-300">
          Valor* {form.type === "percent" ? "(0–100)" : "(R$)"}
          <input
            value={form.value}
            onChange={(e) => setForm({ ...form, value: e.target.value })}
            required
            inputMode="decimal"
            placeholder={form.type === "percent" ? "10" : "5.00"}
            className={`mt-1 ${inputCls}`}
          />
        </label>
        <label className="text-sm text-slate-300">
          Subtotal mínimo (R$)
          <input
            value={form.min_subtotal}
            onChange={(e) => setForm({ ...form, min_subtotal: e.target.value })}
            inputMode="decimal"
            placeholder="0.00"
            className={`mt-1 ${inputCls}`}
          />
        </label>
        <label className="flex items-center gap-2 text-sm text-slate-300 sm:col-span-2">
          <input
            type="checkbox"
            checked={form.active}
            onChange={(e) => setForm({ ...form, active: e.target.checked })}
            className="h-4 w-4 accent-emerald-500"
          />
          Ativo
        </label>
        {formError && (
          <p role="alert" className="text-sm text-red-400 sm:col-span-2">
            {formError}
          </p>
        )}
        <div className="sm:col-span-2">
          <button
            type="submit"
            disabled={saving}
            className="rounded-lg bg-amber-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-amber-400 disabled:opacity-50"
          >
            {saving ? "Salvando…" : editing ? "Salvar alterações" : "Criar cupom"}
          </button>
        </div>
      </form>
    </div>
  );
}
