"use client";

import { useEffect, useState } from "react";
import { api, HttpError } from "@/lib/api";
import type { Category, Product } from "@/lib/api";
import { unwrapList } from "@/lib/admin";
import { formatBRL } from "@/lib/money";

const inputCls =
  "w-full rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white placeholder:text-slate-500";

type Form = {
  name: string;
  description: string;
  price: string;
  stock: string;
  category_id: string;
  image_url: string;
  active: boolean;
};

const emptyForm: Form = {
  name: "",
  description: "",
  price: "",
  stock: "",
  category_id: "",
  image_url: "",
  active: true,
};

export default function AdminProdutos() {
  const [products, setProducts] = useState<Product[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState("");
  const [formError, setFormError] = useState("");
  const [editing, setEditing] = useState<Product | null>(null);
  const [form, setForm] = useState<Form>(emptyForm);
  const [saving, setSaving] = useState(false);

  async function reload() {
    try {
      const res = await api<Product[] | { items: Product[] }>(
        "/api/v1/products?per_page=100&sort=newest",
        { auth: false },
      );
      setProducts(unwrapList(res));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao carregar produtos");
    }
  }

  useEffect(() => {
    reload();
    api<Category[] | { items: Category[] }>("/api/v1/categories", { auth: false })
      .then((res) => setCategories(unwrapList(res)))
      .catch(() => undefined);
  }, []);

  function openNew() {
    setEditing(null);
    setForm(emptyForm);
    setFormError("");
  }

  function openEdit(p: Product) {
    setEditing(p);
    setForm({
      name: p.name,
      description: p.description ?? "",
      price: (p.price_cents / 100).toFixed(2),
      stock: String(p.stock_qty),
      category_id: p.category_id ?? "",
      image_url: p.image_url ?? "",
      active: p.active,
    });
    setFormError("");
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setFormError("");
    // NOTE: create/update accept only known fields (backend DisallowUnknownFields).
    const body: Record<string, unknown> = {
      name: form.name.trim(),
      description: form.description,
      price_cents: Math.round(Number(form.price) * 100),
      stock_qty: Number(form.stock),
      image_url: form.image_url,
      active: form.active,
    };
    if (form.category_id) body.category_id = form.category_id;
    try {
      if (editing) {
        await api(`/api/v1/admin/products/${editing.id}`, { method: "PUT", body });
      } else {
        await api("/api/v1/admin/products", { method: "POST", body });
      }
      openNew();
      await reload();
    } catch (err) {
      setFormError(err instanceof HttpError ? err.message : "Falha ao salvar");
    } finally {
      setSaving(false);
    }
  }

  async function remove(p: Product) {
    if (!window.confirm(`Excluir "${p.name}"?`)) return;
    try {
      await api(`/api/v1/admin/products/${p.id}`, { method: "DELETE" });
      await reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao excluir");
    }
  }

  return (
    <div data-testid="admin-products">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-white">Produtos</h1>
        <button
          data-testid="admin-product-new"
          onClick={openNew}
          className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-500"
        >
          Novo produto
        </button>
      </div>
      {error && (
        <p role="alert" className="mt-3 text-sm text-red-400">
          {error}
        </p>
      )}

      <div className="mt-4 overflow-x-auto rounded-xl border border-slate-800">
        <table className="w-full min-w-[640px] text-left text-sm">
          <thead>
            <tr className="bg-slate-900 text-xs uppercase tracking-wide text-slate-400">
              <th className="px-4 py-3">Nome</th>
              <th className="px-4 py-3">Preço</th>
              <th className="px-4 py-3">Estoque</th>
              <th className="px-4 py-3">Ativo</th>
              <th className="px-4 py-3 text-right">Ações</th>
            </tr>
          </thead>
          <tbody className="bg-slate-900/50">
            {products.map((p) => (
              <tr key={p.id} className="border-t border-slate-800" data-testid="admin-product-row">
                <td className="px-4 py-2 font-medium text-slate-100">{p.name}</td>
                <td className="px-4 py-2 font-mono text-slate-200">{formatBRL(p.price_cents)}</td>
                <td className="px-4 py-2 font-mono text-slate-200">{p.stock_qty}</td>
                <td className="px-4 py-2">{p.active ? "✅" : "🚫"}</td>
                <td className="px-4 py-2 text-right">
                  <button
                    onClick={() => openEdit(p)}
                    className="mr-2 rounded bg-slate-800 px-3 py-1 text-xs text-slate-200 hover:bg-slate-700"
                  >
                    Editar
                  </button>
                  <button
                    onClick={() => remove(p)}
                    className="rounded bg-red-950 px-3 py-1 text-xs text-red-300 hover:bg-red-900"
                  >
                    Excluir
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {products.length === 0 && (
          <p className="bg-slate-900/50 px-4 py-6 text-sm text-slate-400">
            Nenhum produto ativo. A listagem usa o catálogo público (só `active=true` aparece
            aqui); crie ou reative produtos pelo formulário.
          </p>
        )}
      </div>

      <form
        onSubmit={submit}
        data-testid="admin-product-form"
        className="mt-6 grid gap-3 rounded-xl border border-slate-800 bg-slate-900 p-5 sm:grid-cols-2"
      >
        <h2 className="font-semibold text-white sm:col-span-2">
          {editing ? `Editar: ${editing.name}` : "Novo produto"}
        </h2>
        <label className="text-sm text-slate-300">
          Nome*
          <input
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            required
            className={`mt-1 ${inputCls}`}
          />
        </label>
        <label className="text-sm text-slate-300">
          Categoria
          <select
            value={form.category_id}
            onChange={(e) => setForm({ ...form, category_id: e.target.value })}
            className={`mt-1 ${inputCls}`}
          >
            <option value="">Sem categoria</option>
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm text-slate-300 sm:col-span-2">
          Descrição
          <textarea
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            rows={2}
            className={`mt-1 ${inputCls}`}
          />
        </label>
        <label className="text-sm text-slate-300">
          Preço (R$)*
          <input
            value={form.price}
            onChange={(e) => setForm({ ...form, price: e.target.value })}
            required
            inputMode="decimal"
            placeholder="199.90"
            className={`mt-1 ${inputCls}`}
          />
        </label>
        <label className="text-sm text-slate-300">
          Estoque inicial*
          <input
            value={form.stock}
            onChange={(e) => setForm({ ...form, stock: e.target.value })}
            required
            inputMode="numeric"
            placeholder="10"
            className={`mt-1 ${inputCls}`}
          />
        </label>
        <label className="text-sm text-slate-300 sm:col-span-2">
          URL da imagem
          <input
            value={form.image_url}
            onChange={(e) => setForm({ ...form, image_url: e.target.value })}
            className={`mt-1 ${inputCls}`}
          />
        </label>
        <label className="flex items-center gap-2 text-sm text-slate-300">
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
            {saving ? "Salvando…" : editing ? "Salvar alterações" : "Criar produto"}
          </button>
        </div>
      </form>
    </div>
  );
}
