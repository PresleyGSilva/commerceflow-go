"use client";

import { useEffect, useState } from "react";
import { api, HttpError } from "@/lib/api";
import type { Category } from "@/lib/api";
import { unwrapList } from "@/lib/admin";

const inputCls =
  "w-full rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white placeholder:text-slate-500";

export default function AdminCategorias() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState("");
  const [formError, setFormError] = useState("");
  const [name, setName] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");

  async function reload() {
    try {
      const res = await api<Category[] | { items: Category[] }>("/api/v1/categories", {
        auth: false,
      });
      setCategories(unwrapList(res));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao carregar categorias");
    }
  }

  useEffect(() => {
    reload();
  }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    try {
      await api("/api/v1/admin/categories", { method: "POST", body: { name: name.trim() } });
      setName("");
      await reload();
    } catch (err) {
      setFormError(err instanceof HttpError ? err.message : "Falha ao criar");
    }
  }

  async function saveEdit(id: string) {
    setFormError("");
    try {
      await api(`/api/v1/admin/categories/${id}`, {
        method: "PUT",
        body: { name: editName.trim() },
      });
      setEditingId(null);
      await reload();
    } catch (err) {
      setFormError(err instanceof HttpError ? err.message : "Falha ao salvar");
    }
  }

  async function remove(id: string, label: string) {
    if (!window.confirm(`Excluir categoria "${label}"?`)) return;
    try {
      await api(`/api/v1/admin/categories/${id}`, { method: "DELETE" });
      await reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha ao excluir");
    }
  }

  return (
    <div data-testid="admin-categories">
      <h1 className="text-xl font-bold text-white">Categorias</h1>
      {error && (
        <p role="alert" className="mt-3 text-sm text-red-400">
          {error}
        </p>
      )}

      <form
        onSubmit={create}
        data-testid="admin-category-form"
        className="mt-4 flex gap-2 rounded-xl border border-slate-800 bg-slate-900 p-4"
      >
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          placeholder="Nova categoria (ex.: Eletrônicos)"
          aria-label="Nome da categoria"
          className={inputCls}
        />
        <button
          type="submit"
          className="shrink-0 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-500"
        >
          Criar
        </button>
      </form>
      {formError && (
        <p role="alert" className="mt-2 text-sm text-red-400">
          {formError}
        </p>
      )}

      <ul className="mt-4 space-y-2">
        {categories.map((c) => (
          <li
            key={c.id}
            data-testid="admin-category-row"
            className="flex items-center gap-2 rounded-xl border border-slate-800 bg-slate-900 px-4 py-3"
          >
            {editingId === c.id ? (
              <>
                <input
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  aria-label="Editar nome"
                  className={inputCls}
                />
                <button
                  onClick={() => saveEdit(c.id)}
                  className="shrink-0 rounded bg-emerald-600 px-3 py-2 text-xs font-semibold text-white"
                >
                  Salvar
                </button>
                <button
                  onClick={() => setEditingId(null)}
                  className="shrink-0 rounded bg-slate-800 px-3 py-2 text-xs text-slate-300"
                >
                  Cancelar
                </button>
              </>
            ) : (
              <>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-slate-100">{c.name}</p>
                  <p className="font-mono text-xs text-slate-500">{c.slug}</p>
                </div>
                <button
                  onClick={() => {
                    setEditingId(c.id);
                    setEditName(c.name);
                  }}
                  className="shrink-0 rounded bg-slate-800 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-700"
                >
                  Editar
                </button>
                <button
                  onClick={() => remove(c.id, c.name)}
                  className="shrink-0 rounded bg-red-950 px-3 py-1.5 text-xs text-red-300 hover:bg-red-900"
                >
                  Excluir
                </button>
              </>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
