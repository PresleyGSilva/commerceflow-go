"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api, HttpError } from "@/lib/api";
import type { Address } from "@/lib/api";
import { useAuth } from "@/lib/auth";

export default function EnderecosPage() {
  const { user, loading } = useAuth();
  const [items, setItems] = useState<Address[]>([]);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ street: "", city: "", state: "", zip: "" });
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (loading || !user) return;
    api<Address[] | { items: Address[] }>("/api/v1/addresses")
      .then((res) => setItems(Array.isArray(res) ? res : (res.items ?? [])))
      .catch((e) => setError(e instanceof Error ? e.message : "Falha ao carregar"));
  }, [loading, user]);

  if (!loading && !user)
    return (
      <p className="text-sm">
        <Link href="/login" className="text-brand-700 underline">
          Entre
        </Link>{" "}
        para gerenciar endereços.
      </p>
    );

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      const created = await api<Address>("/api/v1/addresses", {
        method: "POST",
        body: { ...form, country: "BR" },
      });
      setItems((prev) => [...prev, created]);
      setForm({ street: "", city: "", state: "", zip: "" });
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Falha ao salvar");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold">Endereços</h1>
      {error && (
        <p className="mt-3 text-sm text-red-600" role="alert">
          {error}
        </p>
      )}
      <ul className="mt-6 space-y-2">
        {items.map((a) => (
          <li key={a.id} className="rounded-xl border border-stone-200 bg-white p-4 text-sm">
            {a.street} — {a.city}/{a.state} — {a.zip}
          </li>
        ))}
      </ul>
      <form onSubmit={create} className="mt-6 space-y-3 rounded-xl border border-stone-200 bg-white p-5">
        <h2 className="font-semibold">Novo endereço</h2>
        <div>
          <label htmlFor="ad-street" className="text-sm font-medium">
            Rua e número
          </label>
          <input
            id="ad-street"
            required
            value={form.street}
            onChange={(e) => setForm({ ...form, street: e.target.value })}
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
          />
        </div>
        <div className="grid grid-cols-3 gap-3">
          <div>
            <label htmlFor="ad-city" className="text-sm font-medium">
              Cidade
            </label>
            <input
              id="ad-city"
              required
              value={form.city}
              onChange={(e) => setForm({ ...form, city: e.target.value })}
              className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
            />
          </div>
          <div>
            <label htmlFor="ad-state" className="text-sm font-medium">
              UF
            </label>
            <input
              id="ad-state"
              required
              value={form.state}
              onChange={(e) => setForm({ ...form, state: e.target.value })}
              className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
            />
          </div>
          <div>
            <label htmlFor="ad-zip" className="text-sm font-medium">
              CEP
            </label>
            <input
              id="ad-zip"
              required
              value={form.zip}
              onChange={(e) => setForm({ ...form, zip: e.target.value })}
              className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2"
            />
          </div>
        </div>
        <button
          type="submit"
          disabled={saving}
          className="rounded-lg bg-brand-600 px-5 py-2 font-semibold text-white hover:bg-brand-700 disabled:bg-stone-300"
        >
          {saving ? "Salvando…" : "Salvar endereço"}
        </button>
      </form>
    </div>
  );
}
