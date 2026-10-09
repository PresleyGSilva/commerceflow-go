"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import StatusBadge from "@/components/StatusBadge";
import { api } from "@/lib/api";
import type { Order } from "@/lib/api";
import { useAuth } from "@/lib/auth";

export default function PedidosPage() {
  const { user, loading } = useAuth();
  const [orders, setOrders] = useState<Order[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (loading || !user) return;
    api<Order[] | { items: Order[] }>("/api/v1/orders")
      .then((res) => setOrders(Array.isArray(res) ? res : (res.items ?? [])))
      .catch((e) => setError(e instanceof Error ? e.message : "Falha ao carregar pedidos"));
  }, [loading, user]);

  if (!loading && !user)
    return (
      <p className="text-sm">
        <Link href="/login" className="text-brand-700 underline">
          Entre
        </Link>{" "}
        para ver seus pedidos.
      </p>
    );

  return (
    <div>
      <h1 className="text-2xl font-bold">Meus pedidos</h1>
      {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
      <ul className="mt-6 space-y-3">
        {orders.map((o) => (
          <li key={o.id} className="rounded-xl border border-stone-200 bg-white p-4">
            <Link href={`/pedidos/${o.id}`} className="flex items-center justify-between">
              <span className="font-mono text-sm">{o.id.slice(0, 8)}…</span>
              <StatusBadge status={o.status} />
              <strong>
                {(o.total_cents / 100).toLocaleString("pt-BR", {
                  style: "currency",
                  currency: "BRL",
                })}
              </strong>
            </Link>
          </li>
        ))}
      </ul>
      {orders.length === 0 && !error && (
        <p className="mt-6 text-sm text-stone-500">Nenhum pedido ainda.</p>
      )}
    </div>
  );
}
