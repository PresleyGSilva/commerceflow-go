// Admin API shapes (backend: internal/admin + catalog admin routes).
// All money is integer cents. Envelopes are {data} / {error:{code,message}}.

export type LowStockProduct = {
  id: string;
  name: string;
  slug: string;
  stock_qty: number;
  price_cents: number;
};

export type TopProduct = {
  id: string;
  name: string;
  qty: number;
  revenue_cents: number;
};

export type DashboardData = {
  revenue_cents: number;
  orders_count: number;
  paid_count: number;
  low_stock: LowStockProduct[];
  top_products: TopProduct[];
};

export type SalesPoint = {
  date: string;
  orders_count: number;
  revenue_cents: number;
};

export type SalesReport = {
  from: string;
  to: string;
  group_by: string;
  points: SalesPoint[];
};

export type StockItem = {
  id: string;
  name: string;
  slug: string;
  stock_qty: number;
  price_cents: number;
  stock_value_cents: number;
  low_stock: boolean;
};

export type Coupon = {
  id: string;
  code: string;
  type: string;
  value: number;
  min_subtotal_cents: number;
  active: boolean;
  expires_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type StockMovement = {
  id: string;
  product_id: string;
  product_slug: string;
  product_name: string;
  delta: number;
  reason: string;
  created_at: string;
};

export type AdminOrderItem = {
  product_id: string;
  product_name: string;
  qty: number;
  unit_price_cents: number;
};

export type AdminOrder = {
  id: string;
  user_id?: string;
  status: string;
  subtotal_cents: number;
  discount_cents: number;
  total_cents: number;
  coupon_code: string | null;
  items?: AdminOrderItem[];
  created_at: string;
};

export function unwrapList<T>(v: T[] | { items: T[] } | null | undefined): T[] {
  if (!v) return [];
  return Array.isArray(v) ? v : (v.items ?? []);
}
