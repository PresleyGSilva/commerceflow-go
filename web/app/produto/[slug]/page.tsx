"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Product } from "@/lib/api";
import { useCart } from "@/lib/cart";

export default function ProdutoPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = use(params);
  const [product, setProduct] = useState<Product | null>(null);
  const [qty, setQty] = useState(1);
  const [error, setError] = useState("");
  const { add } = useCart();

  useEffect(() => {
    api<Product | { product: Product }>(`/api/v1/products/${slug}`, { auth: false })
      .then((res) => {
        if (res && typeof res === "object" && "product" in res) {
          const inner: unknown = res.product;
          if (inner && typeof inner === "object") setProduct(inner as Product);
        } else {
          setProduct(res as Product);
        }
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Produto não encontrado"));
  }, [slug]);

  if (error)
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        {error} — <Link href="/catalogo" className="underline">voltar ao catálogo</Link>
      </div>
    );
  if (!product) return <p className="text-sm text-stone-500">Carregando…</p>;

  return (
    <div className="grid gap-8 md:grid-cols-2">
      <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
        {product.image_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={product.image_url} alt={product.name} className="aspect-square w-full object-cover" />
        ) : (
          <span className="grid aspect-square place-items-center text-7xl" aria-hidden>
            📦
          </span>
        )}
      </div>
      <div>
        <Link href="/catalogo" className="text-sm text-brand-700 hover:underline">
          ← Voltar
        </Link>
        <h1 className="mt-2 text-3xl font-bold">{product.name}</h1>
        <p className="mt-3 text-3xl font-bold text-brand-700">
          {(product.price_cents / 100).toLocaleString("pt-BR", {
            style: "currency",
            currency: "BRL",
          })}
        </p>
        <p className="mt-1 text-sm text-stone-500">
          {product.stock_qty > 0 ? `${product.stock_qty} em estoque` : "Esgotado"}
        </p>
        {product.description && <p className="mt-4 text-stone-700">{product.description}</p>}
        <div className="mt-6 flex items-center gap-3">
          <label htmlFor="qty" className="text-sm font-medium">
            Qtd
          </label>
          <input
            id="qty"
            type="number"
            min={1}
            max={Math.max(1, product.stock_qty)}
            value={qty}
            onChange={(e) => setQty(Math.max(1, Number(e.target.value) || 1))}
            className="w-20 rounded-lg border border-stone-300 px-3 py-2"
          />
          <button
            data-testid="add-to-cart"
            disabled={product.stock_qty <= 0}
            onClick={() => add(product, qty)}
            className="rounded-lg bg-brand-600 px-6 py-2.5 font-semibold text-white hover:bg-brand-700 disabled:bg-stone-300"
          >
            Adicionar ao carrinho
          </button>
        </div>
      </div>
    </div>
  );
}
