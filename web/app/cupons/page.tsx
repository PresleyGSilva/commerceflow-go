"use client";

import { useState } from "react";
import { api, HttpError } from "@/lib/api";
import type { CouponPreview } from "@/lib/api";

export default function CuponsPage() {
  const [code, setCode] = useState("");
  const [result, setResult] = useState<CouponPreview | null>(null);
  const [error, setError] = useState("");
  const [checking, setChecking] = useState(false);

  async function validate(e: React.FormEvent) {
    e.preventDefault();
    setChecking(true);
    setError("");
    setResult(null);
    try {
      const res = await api<CouponPreview | { coupon: CouponPreview }>(
        `/api/v1/coupons/validate?code=${encodeURIComponent(code.trim().toUpperCase())}`,
      );
      if (res && typeof res === "object" && "coupon" in res) {
        const inner: unknown = res.coupon;
        if (inner && typeof inner === "object") setResult(inner as CouponPreview);
      } else {
        setResult(res as CouponPreview);
      }
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Cupom inválido");
    } finally {
      setChecking(false);
    }
  }

  return (
    <div className="max-w-xl">
      <h1 className="text-2xl font-bold">Validar cupom</h1>
      <p className="mt-2 text-sm text-stone-600">
        Cupons demo: <strong>BEMVINDO10</strong> (10% off) e <strong>FRETE5</strong> (R$ 5,00 off).
      </p>
      <form onSubmit={validate} className="mt-4 flex gap-2">
        <label htmlFor="coupon-code" className="sr-only">
          Código do cupom
        </label>
        <input
          id="coupon-code"
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          placeholder="BEMVINDO10"
          required
          className="flex-1 rounded-lg border border-stone-300 px-3 py-2 uppercase"
        />
        <button
          type="submit"
          disabled={checking}
          className="rounded-lg bg-brand-600 px-5 py-2 font-semibold text-white hover:bg-brand-700 disabled:bg-stone-300"
        >
          {checking ? "…" : "Validar"}
        </button>
      </form>
      {error && (
        <p className="mt-3 text-sm text-red-600" role="alert">
          {error}
        </p>
      )}
      {result && (
        <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50 p-5 text-sm">
          <p className="font-bold text-emerald-800">{result.code} válido ✓</p>
          <p className="mt-1 text-emerald-700">
            {result.type === "percent"
              ? `${result.value}% de desconto`
              : `${(result.value / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })} de desconto`}
            {result.min_subtotal_cents > 0 &&
              ` em compras acima de ${(result.min_subtotal_cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" })}`}
            .
          </p>
        </div>
      )}
    </div>
  );
}
