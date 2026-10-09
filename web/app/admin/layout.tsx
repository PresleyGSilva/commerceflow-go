"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { useAuth } from "@/lib/auth";

const NAV = [
  { href: "/admin", label: "Dashboard", testid: "admin-nav-dashboard" },
  { href: "/admin/produtos", label: "Produtos", testid: "admin-nav-produtos" },
  { href: "/admin/categorias", label: "Categorias", testid: "admin-nav-categorias" },
  { href: "/admin/estoque", label: "Estoque", testid: "admin-nav-estoque" },
  { href: "/admin/pedidos", label: "Pedidos", testid: "admin-nav-pedidos" },
  { href: "/admin/cupons", label: "Cupons", testid: "admin-nav-cupons" },
  { href: "/admin/relatorios", label: "Relatórios", testid: "admin-nav-relatorios" },
];

// Guard: admin role only; redirects to /login otherwise.
// Renders the slate admin shell around every /admin/* page.
export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!loading && (!user || user.role !== "admin")) router.replace("/login");
  }, [loading, user, router]);

  if (loading) {
    return (
      <div className="rounded-xl bg-slate-900 p-8 text-slate-300" data-testid="admin-loading">
        Carregando painel…
      </div>
    );
  }
  if (!user || user.role !== "admin") {
    return (
      <div className="rounded-xl bg-slate-900 p-8 text-slate-300" data-testid="admin-forbidden">
        <p className="font-semibold text-white">Acesso restrito</p>
        <p className="mt-1 text-sm">Esta área é exclusiva para administradores.</p>
        <Link href="/login" className="mt-4 inline-block text-sm text-emerald-400 underline">
          Ir para o login
        </Link>
      </div>
    );
  }

  return (
    <div data-testid="admin-shell" className="overflow-hidden rounded-2xl bg-slate-950 text-slate-100">
      <div className="flex items-center justify-between border-b border-slate-800 px-5 py-4">
        <p className="text-sm font-bold uppercase tracking-widest">
          <span className="text-emerald-400">CommerceFlow</span>{" "}
          <span className="text-slate-400">Admin</span>
        </p>
        <p className="hidden text-xs text-slate-400 sm:block" data-testid="admin-user">
          {user.email}
        </p>
      </div>
      <nav
        aria-label="Administração"
        className="flex gap-1 overflow-x-auto border-b border-slate-800 bg-slate-900 px-3 py-2"
      >
        {NAV.map((item) => {
          const active =
            item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              data-testid={item.testid}
              aria-current={active ? "page" : undefined}
              className={`whitespace-nowrap rounded-lg px-3 py-2 text-sm font-medium transition ${
                active
                  ? "bg-emerald-500/15 text-emerald-300"
                  : "text-slate-300 hover:bg-slate-800 hover:text-white"
              }`}
            >
              {item.label}
            </Link>
          );
        })}
      </nav>
      <div className="bg-slate-950 p-5">{children}</div>
    </div>
  );
}
