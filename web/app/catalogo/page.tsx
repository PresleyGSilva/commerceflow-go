"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import ProductCard from "@/components/ProductCard";
import { api } from "@/lib/api";
import type { Category, Product } from "@/lib/api";

type Paged<T> = { items: T[]; total: number; page: number; per_page: number };

function unwrap<T>(v: Paged<T> | T[]): T[] {
  return Array.isArray(v) ? v : (v.items ?? []);
}

export default function CatalogoPage() {
  return (
    <Suspense fallback={<p className="text-sm text-stone-500">Carregando…</p>}>
      <CatalogoInner />
    </Suspense>
  );
}

function CatalogoInner() {
  const params = useSearchParams();
  const [products, setProducts] = useState<Product[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [q, setQ] = useState(params.get("q") ?? "");
  const [categoria, setCategoria] = useState(params.get("categoria") ?? "");
  const [min, setMin] = useState("");
  const [max, setMax] = useState("");
  const [ordem, setOrdem] = useState("newest");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api<Paged<Category> | Category[]>("/api/v1/categories", { auth: false })
      .then((cats) => setCategories(unwrap(cats)))
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    setLoading(true);
    setError("");
    const qs = new URLSearchParams();
    if (q) qs.set("q", q);
    if (categoria) qs.set("category", categoria);
    if (min) qs.set("min_price", String(Math.round(Number(min) * 100)));
    if (max) qs.set("max_price", String(Math.round(Number(max) * 100)));
    if (ordem) qs.set("sort", ordem);
    qs.set("page", String(page));
    qs.set("limit", "12");
    api<Paged<Product> | Product[]>(`/api/v1/products?${qs.toString()}`, { auth: false })
      .then((res) => {
        setProducts(unwrap(res).filter((p) => p.active !== false));
        setTotal(Array.isArray(res) ? res.length : (res.total ?? 0));
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Falha ao carregar"))
      .finally(() => setLoading(false));
  }, [q, categoria, min, max, ordem, page]);

  return (
    <div>
      <h1 className="text-2xl font-bold">Catálogo</h1>
      <form
        className="mt-4 grid gap-3 rounded-xl border border-stone-200 bg-white p-4 md:grid-cols-6"
        onSubmit={(e) => e.preventDefault()}
        aria-label="Filtros do catálogo"
      >
        <div className="md:col-span-2">
          <label htmlFor="f-q" className="text-xs font-semibold">
            Buscar
          </label>
          <input
            id="f-q"
            value={q}
            onChange={(e) => {
              setPage(1);
              setQ(e.target.value);
            }}
            placeholder="Nome do produto…"
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label htmlFor="f-cat" className="text-xs font-semibold">
            Categoria
          </label>
          <select
            id="f-cat"
            value={categoria}
            onChange={(e) => {
              setPage(1);
              setCategoria(e.target.value);
            }}
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2 text-sm"
          >
            <option value="">Todas</option>
            {categories.map((c) => (
              <option key={c.id} value={c.slug}>
                {c.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label htmlFor="f-min" className="text-xs font-semibold">
            Preço mín (R$)
          </label>
          <input
            id="f-min"
            inputMode="decimal"
            value={min}
            onChange={(e) => {
              setPage(1);
              setMin(e.target.value);
            }}
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label htmlFor="f-max" className="text-xs font-semibold">
            Preço máx (R$)
          </label>
          <input
            id="f-max"
            inputMode="decimal"
            value={max}
            onChange={(e) => {
              setPage(1);
              setMax(e.target.value);
            }}
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label htmlFor="f-ord" className="text-xs font-semibold">
            Ordenar
          </label>
          <select
            id="f-ord"
            value={ordem}
            onChange={(e) => {
              setPage(1);
              setOrdem(e.target.value);
            }}
            className="mt-1 w-full rounded-lg border border-stone-300 px-3 py-2 text-sm"
          >
            <option value="newest">Novidades</option>
            <option value="price_asc">Menor preço</option>
            <option value="price_desc">Maior preço</option>
            <option value="name">Nome A–Z</option>
          </select>
        </div>
      </form>

      {loading && <p className="mt-6 text-sm text-stone-500">Carregando…</p>}
      {error && <p className="mt-6 text-sm text-red-600">{error}</p>}
      {!loading && !error && products.length === 0 && (
        <p className="mt-6 text-sm text-stone-500">Nenhum produto encontrado.</p>
      )}
      <div className="mt-6 grid grid-cols-2 gap-4 md:grid-cols-4">
        {products.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>
      {total > 12 && (
        <div className="mt-8 flex items-center justify-center gap-2">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
            className="rounded-lg border border-stone-300 px-4 py-2 text-sm disabled:opacity-50"
          >
            ← Anterior
          </button>
          <span className="text-sm text-stone-600">Página {page}</span>
          <button
            onClick={() => setPage((p) => p + 1)}
            disabled={products.length < 12}
            className="rounded-lg border border-stone-300 px-4 py-2 text-sm disabled:opacity-50"
          >
            Próxima →
          </button>
        </div>
      )}
    </div>
  );
}
