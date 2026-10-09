"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import ProductCard from "@/components/ProductCard";
import { api, type Category, type Product } from "@/lib/api";

type Paged<T> = { items: T[]; total?: number };

export default function HomePage() {
  const [featured, setFeatured] = useState<Product[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const [prods, cats] = await Promise.all([
          api<Paged<Product> | Product[]>("/api/v1/products?limit=8", { auth: false }),
          api<Paged<Category> | Category[]>("/api/v1/categories", { auth: false }),
        ]);
        if (!alive) return;
        setFeatured(Array.isArray(prods) ? prods.slice(0, 8) : (prods.items ?? []).slice(0, 8));
        setCategories(Array.isArray(cats) ? cats : (cats.items ?? []));
      } catch (e) {
        if (alive) setError(e instanceof Error ? e.message : "Falha ao carregar");
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div className="space-y-12">
      <section className="overflow-hidden rounded-2xl bg-gradient-to-r from-brand-600 to-emerald-700 px-8 py-14 text-white">
        <h1 className="max-w-xl text-4xl font-bold">
          Tudo que você precisa, com checkout em segundos.
        </h1>
        <p className="mt-3 max-w-lg text-emerald-50">
          Eletrônicos, moda, casa e esporte. Use o cupom{" "}
          <strong className="rounded bg-accent-500 px-2 py-0.5 text-white">BEMVINDO10</strong> e ganhe
          10% off.
        </p>
        <div className="mt-6 flex gap-3">
          <Link
            href="/catalogo"
            className="rounded-lg bg-white px-5 py-2.5 font-semibold text-brand-700 hover:bg-emerald-50"
          >
            Ver catálogo
          </Link>
          <Link
            href="/cupons"
            className="rounded-lg border border-white/40 px-5 py-2.5 font-semibold hover:bg-white/10"
          >
            Meus cupons
          </Link>
        </div>
      </section>

      {categories.length > 0 && (
        <section aria-label="Categorias">
          <h2 className="text-xl font-bold">Comprar por categoria</h2>
          <div className="mt-4 flex flex-wrap gap-2">
            {categories.map((c) => (
              <Link
                key={c.id}
                href={`/catalogo?categoria=${encodeURIComponent(c.slug)}`}
                className="rounded-full border border-stone-300 bg-white px-4 py-2 text-sm hover:border-brand-600 hover:text-brand-700"
              >
                {c.name}
              </Link>
            ))}
          </div>
        </section>
      )}

      <section aria-label="Destaques">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-bold">Destaques</h2>
          <Link href="/catalogo" className="text-sm font-medium text-brand-700 hover:underline">
            Ver tudo →
          </Link>
        </div>
        {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
        <div className="mt-4 grid grid-cols-2 gap-4 md:grid-cols-4">
          {featured.map((p) => (
            <ProductCard key={p.id} product={p} />
          ))}
        </div>
      </section>
    </div>
  );
}
