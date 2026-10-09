"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { Product } from "./api";

export type CartLine = { product: Product; qty: number };

const CART_KEY = "commerceflow_cart";

type CartState = {
  lines: CartLine[];
  count: number;
  subtotalCents: number;
  add: (p: Product, qty?: number) => void;
  remove: (id: string) => void;
  setQty: (id: string, qty: number) => void;
  clear: () => void;
};

const CartContext = createContext<CartState | null>(null);

function load(): CartLine[] {
  try {
    const raw = window.localStorage.getItem(CART_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as CartLine[];
    return Array.isArray(parsed) ? parsed.filter((l) => l?.product?.id && l.qty > 0) : [];
  } catch {
    return [];
  }
}

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [lines, setLines] = useState<CartLine[]>([]);

  useEffect(() => {
    setLines(load());
  }, []);

  useEffect(() => {
    try {
      window.localStorage.setItem(CART_KEY, JSON.stringify(lines));
    } catch {
      // storage full/blocked: cart just won't persist
    }
  }, [lines]);

  const value = useMemo<CartState>(() => {
    const count = lines.reduce((n, l) => n + l.qty, 0);
    const subtotalCents = lines.reduce((n, l) => n + l.qty * l.product.price_cents, 0);
    return {
      lines,
      count,
      subtotalCents,
      add: (p, qty = 1) =>
        setLines((prev) => {
          const found = prev.find((l) => l.product.id === p.id);
          if (found)
            return prev.map((l) =>
              l.product.id === p.id ? { ...l, qty: Math.min(l.qty + qty, 99) } : l,
            );
          return [...prev, { product: p, qty }];
        }),
      remove: (id) => setLines((prev) => prev.filter((l) => l.product.id !== id)),
      setQty: (id, qty) =>
        setLines((prev) =>
          qty <= 0
            ? prev.filter((l) => l.product.id !== id)
            : prev.map((l) => (l.product.id === id ? { ...l, qty: Math.min(qty, 99) } : l)),
        ),
      clear: () => setLines([]),
    };
  }, [lines]);

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart(): CartState {
  const ctx = useContext(CartContext);
  if (!ctx) throw new Error("useCart must be used within CartProvider");
  return ctx;
}
