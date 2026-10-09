"use client";

import Link from "next/link";
import type { Product } from "@/lib/api";
import { useCart } from "@/lib/cart";

function categoryLabel(p: Product): string {
  if (!p.category) return "";
  return typeof p.category === "string" ? p.category : p.category.name;
}

export default function ProductCard({ product }: { product: Product }) {
  const { add } = useCart();
  return (
    <article
      data-testid="product-card"
      className="flex flex-col overflow-hidden rounded-xl border border-stone-200 bg-white transition hover:shadow-md"
    >
      <Link
        href={`/produto/${product.slug}`}
        className="block aspect-square bg-stone-100"
        aria-label={`Ver ${product.name}`}
      >
        {product.image_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={product.image_url} alt={product.name} className="h-full w-full object-cover" />
        ) : (
          <span className="grid h-full w-full place-items-center text-4xl" aria-hidden>
            📦
          </span>
        )}
      </Link>
      <div className="flex flex-1 flex-col gap-1 p-4">
        {categoryLabel(product) && (
          <p className="text-xs font-medium uppercase tracking-wide text-brand-600">
            {categoryLabel(product)}
          </p>
        )}
        <Link href={`/produto/${product.slug}`} className="font-semibold hover:text-brand-700">
          {product.name}
        </Link>
        <p className="text-lg font-bold text-stone-900">
          {(product.price_cents / 100).toLocaleString("pt-BR", {
            style: "currency",
            currency: "BRL",
          })}
        </p>
        <button
          data-testid="add-to-cart"
          onClick={() => add(product)}
          disabled={product.stock_qty <= 0}
          className="mt-auto rounded-lg bg-brand-600 px-3 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:bg-stone-300"
        >
          {product.stock_qty > 0 ? "Adicionar" : "Esgotado"}
        </button>
      </div>
    </article>
  );
}
