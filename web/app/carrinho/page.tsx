"use client";

import Link from "next/link";
import { useCart } from "@/lib/cart";

export default function CarrinhoPage() {
  const { lines, subtotalCents, setQty, remove, clear } = useCart();

  if (lines.length === 0)
    return (
      <div className="rounded-2xl border border-stone-200 bg-white p-10 text-center">
        <p className="text-lg font-semibold">Seu carrinho está vazio</p>
        <Link
          href="/catalogo"
          className="mt-4 inline-block rounded-lg bg-brand-600 px-6 py-2.5 font-semibold text-white hover:bg-brand-700"
        >
          Explorar catálogo
        </Link>
      </div>
    );

  return (
    <div className="grid gap-8 md:grid-cols-3">
      <ul className="space-y-3 md:col-span-2" aria-label="Itens do carrinho">
        {lines.map((l) => (
          <li
            key={l.product.id}
            className="flex items-center gap-4 rounded-xl border border-stone-200 bg-white p-4"
          >
            <span className="grid h-16 w-16 shrink-0 place-items-center rounded-lg bg-stone-100 text-2xl" aria-hidden>
              📦
            </span>
            <div className="min-w-0 flex-1">
              <Link href={`/produto/${l.product.slug}`} className="font-semibold hover:text-brand-700">
                {l.product.name}
              </Link>
              <p className="text-sm text-stone-500">
                {(l.product.price_cents / 100).toLocaleString("pt-BR", {
                  style: "currency",
                  currency: "BRL",
                })}
              </p>
            </div>
            <label className="sr-only" htmlFor={`qty-${l.product.id}`}>
              Quantidade de {l.product.name}
            </label>
            <input
              id={`qty-${l.product.id}`}
              type="number"
              min={0}
              max={99}
              value={l.qty}
              onChange={(e) => setQty(l.product.id, Number(e.target.value) || 0)}
              className="w-16 rounded-lg border border-stone-300 px-2 py-1 text-center"
            />
            <button
              onClick={() => remove(l.product.id)}
              className="rounded px-2 py-1 text-sm text-red-600 hover:bg-red-50"
              aria-label={`Remover ${l.product.name}`}
            >
              Remover
            </button>
          </li>
        ))}
      </ul>
      <aside className="h-fit rounded-xl border border-stone-200 bg-white p-6" aria-label="Resumo">
        <h2 className="font-bold">Resumo</h2>
        <p className="mt-3 flex justify-between text-sm">
          <span>Subtotal</span>
          <strong>
            {(subtotalCents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })}
          </strong>
        </p>
        <Link
          href="/checkout"
          data-testid="checkout-submit"
          className="mt-5 block rounded-lg bg-brand-600 px-4 py-2.5 text-center font-semibold text-white hover:bg-brand-700"
        >
          Ir para o checkout
        </Link>
        <button
          onClick={clear}
          className="mt-2 w-full rounded-lg border border-stone-300 px-4 py-2 text-sm hover:bg-stone-50"
        >
          Limpar carrinho
        </button>
      </aside>
    </div>
  );
}
