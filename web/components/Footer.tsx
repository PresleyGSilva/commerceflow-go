import Link from "next/link";

export default function Footer() {
  return (
    <footer className="mt-16 border-t border-stone-200 bg-white">
      <div className="mx-auto grid max-w-6xl gap-8 px-4 py-10 md:grid-cols-3">
        <div>
          <p className="font-bold">
            Commerce<span className="text-brand-600">Flow</span>
          </p>
          <p className="mt-2 text-sm text-stone-600">
            Loja demo full-stack: Go + Next.js + Postgres. Preços em centavos, checkout idempotente.
          </p>
        </div>
        <nav aria-label="Links da loja">
          <p className="text-sm font-semibold">Loja</p>
          <ul className="mt-2 space-y-1 text-sm text-stone-600">
            <li>
              <Link href="/catalogo" className="hover:text-brand-700">
                Catálogo
              </Link>
            </li>
            <li>
              <Link href="/carrinho" className="hover:text-brand-700">
                Carrinho
              </Link>
            </li>
            <li>
              <Link href="/cupons" className="hover:text-brand-700">
                Cupons
              </Link>
            </li>
          </ul>
        </nav>
        <nav aria-label="Conta">
          <p className="text-sm font-semibold">Conta</p>
          <ul className="mt-2 space-y-1 text-sm text-stone-600">
            <li>
              <Link href="/login" className="hover:text-brand-700">
                Entrar
              </Link>
            </li>
            <li>
              <Link href="/registro" className="hover:text-brand-700">
                Criar conta
              </Link>
            </li>
            <li>
              <Link href="/pedidos" className="hover:text-brand-700">
                Meus pedidos
              </Link>
            </li>
          </ul>
        </nav>
      </div>
    </footer>
  );
}
