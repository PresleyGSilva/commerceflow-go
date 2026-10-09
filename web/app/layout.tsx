import type { Metadata } from "next";
import { AuthProvider } from "@/lib/auth";
import { CartProvider } from "@/lib/cart";
import Header from "@/components/Header";
import Footer from "@/components/Footer";
import "./globals.css";

export const metadata: Metadata = {
  title: "CommerceFlow — Loja",
  description: "Loja demo CommerceFlow: catálogo, carrinho e checkout idempotente.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="pt-BR">
      <body>
        <AuthProvider>
          <CartProvider>
            <Header />
            <main className="mx-auto min-h-[70vh] max-w-6xl px-4 py-8">{children}</main>
            <Footer />
          </CartProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
