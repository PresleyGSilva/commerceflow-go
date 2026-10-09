"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/lib/auth";
import { useCart } from "@/lib/cart";

function AdminHeader() {
  const { user, logout } = useAuth();
  return (
    <header className="sticky top-0 z-40 border-b border-slate-800 bg-slate-950/95 text-slate-100 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center gap-4 px-4 py-3">
        <Link href="/admin" className="flex items-center gap-2" aria-label="CommerceFlow admin">
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-emerald-600 font-bold text-white">
            C
          </span>
          <span className="text-lg font-bold">
            Commerce<span className="text-emerald-400">Flow</span>{" "}
            <span className="text-sm font-medium text-slate-400">Admin</span>
          </span>
        </Link>
        <nav className="ml-auto flex items-center gap-1 text-sm" aria-label="Admin">
          <Link href="/" className="rounded px-3 py-2 text-slate-300 hover:bg-slate-800">
            Ver loja
          </Link>
          {user ? (
            <button
              onClick={logout}
              className="rounded px-3 py-2 text-slate-300 hover:bg-slate-800"
              aria-label="Sair"
            >
              Sair
            </button>
          ) : (
            <Link
              href="/login"
              className="rounded-lg bg-emerald-600 px-4 py-2 font-medium text-white hover:bg-emerald-500"
            >
              Entrar
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}

export default function Header() {
  const pathname = usePathname();
  if (pathname === "/admin" || pathname.startsWith("/admin/")) return <AdminHeader />;
  return <StoreHeader />;
}

function StoreHeader() {
  const { user, logout } = useAuth();
  const { count } = useCart();
  const [q, setQ] = useState("");
  const router = useRouter();

  return (
    <header className="sticky top-0 z-40 border-b border-stone-200 bg-white/95 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center gap-4 px-4 py-3">
        <Link href="/" className="flex items-center gap-2" aria-label="CommerceFlow home">
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-brand-600 font-bold text-white">
            C
          </span>
          <span className="text-lg font-bold text-stone-900">
            Commerce<span className="text-brand-600">Flow</span>
          </span>
        </Link>
        <form
          role="search"
          className="hidden flex-1 items-center md:flex"
          onSubmit={(e) => {
            e.preventDefault();
            router.push(`/catalogo?q=${encodeURIComponent(q)}`);
          }}
        >
          <label htmlFor="site-search" className="sr-only">
            Buscar produtos
          </label>
          <input
            id="site-search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Buscar produtos…"
            className="w-full rounded-lg border border-stone-300 px-3 py-2 text-sm focus:border-brand-600"
          />
        </form>
        <nav className="ml-auto flex items-center gap-1 text-sm" aria-label="Navegação principal">
          <Link href="/catalogo" className="rounded px-3 py-2 hover:bg-stone-100">
            Catálogo
          </Link>
          <Link href="/cupons" className="rounded px-3 py-2 hover:bg-stone-100">
            Cupons
          </Link>
          {user ? (
            <>
              <Link href="/pedidos" className="rounded px-3 py-2 hover:bg-stone-100">
                Pedidos
              </Link>
              <button
                onClick={logout}
                className="rounded px-3 py-2 text-stone-600 hover:bg-stone-100"
                aria-label="Sair"
              >
                Sair
              </button>
            </>
          ) : (
            <Link
              href="/login"
              className="rounded-lg bg-brand-600 px-4 py-2 font-medium text-white hover:bg-brand-700"
            >
              Entrar
            </Link>
          )}
          <Link
            href="/carrinho"
            className="relative rounded px-3 py-2 hover:bg-stone-100"
            aria-label={`Carrinho, ${count} itens`}
            data-testid="cart-link"
          >
            🛒
            {count > 0 && (
              <span className="absolute -right-0 -top-0 grid h-5 min-w-5 place-items-center rounded-full bg-accent-500 px-1 text-[11px] font-bold text-white">
                {count}
              </span>
            )}
          </Link>
        </nav>
      </div>
    </header>
  );
}
