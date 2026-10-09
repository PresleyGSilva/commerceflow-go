"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Order } from "@/lib/api";
import StatusBadge from "@/components/StatusBadge";

export default function PedidoDetalhePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [order, setOrder] = useState<Order | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api<Order | { order: Order }>(`/api/v1/orders/${id}`)
      .then((res) => {
        if (res && typeof res === "object" && "order" in res) {
          const inner: unknown = res.order;
          if (inner && typeof inner === "object") setOrder(inner as Order);
        } else {
          setOrder(res as Order);
        }
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Pedido não encontrado"));
  }, [id]);

  if (error)
    return (
      <p className="text-sm text-red-600">
        {error} — <Link href="/pedidos" className="underline">voltar</Link>
      </p>
    );
  if (!order) return <p className="text-sm text-stone-500">Carregando…</p>;

  return (
    <div className="max-w-2xl">
      <Link href="/pedidos" className="text-sm text-brand-700 hover:underline">
        ← Pedidos
      </Link>
      <div className="mt-2 flex items-center gap-3">
        <h1 className="font-mono text-xl font-bold">Pedido {order.id.slice(0, 8)}…</h1>
        <StatusBadge status={order.status} />
      </div>
      <ul className="mt-6 space-y-2 rounded-xl border border-stone-200 bg-white p-5">
        {(order.items ?? []).map((it, i) => (
          <li key={i} className="flex justify-between text-sm">
            <span>
              {it.product_name} × {it.qty}
            </span>
            <strong>
              {((it.unit_price_cents * it.qty) / 100).toLocaleString("pt-BR", {
                style: "currency",
                currency: "BRL",
              })}
            </strong>
          </li>
        ))}
      </ul>
      <dl className="mt-4 space-y-1 rounded-xl border border-stone-200 bg-white p-5 text-sm">
        <div className="flex justify-between">
          <dt>Subtotal</dt>
          <dd>
            {(order.subtotal_cents / 100).toLocaleString("pt-BR", {
              style: "currency",
              currency: "BRL",
            })}
          </dd>
        </div>
        <div className="flex justify-between">
          <dt>Desconto {order.coupon_code ? `(${order.coupon_code})` : ""}</dt>
          <dd>
            −{(order.discount_cents / 100).toLocaleString("pt-BR", {
              style: "currency",
              currency: "BRL",
            })}
          </dd>
        </div>
        <div className="flex justify-between font-bold">
          <dt>Total</dt>
          <dd>
            {(order.total_cents / 100).toLocaleString("pt-BR", {
              style: "currency",
              currency: "BRL",
            })}
          </dd>
        </div>
      </dl>
    </div>
  );
}
