// Typed API client: envelope {data} / {error:{code,message}}, JWT + tenant headers,
// automatic single retry via /auth/refresh on 401.

export const API_URL =
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "http://localhost:8082";

export const TOKEN_KEY = "commerceflow_token";
export const ORG_KEY = "commerceflow_org_id";
export const USER_KEY = "commerceflow_user";

export type ApiError = { code: string; message: string };

export class HttpError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

function storageGet(key: string): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function getStoredUser(): { id?: string } | null {
  const raw = storageGet(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

type Options = Omit<RequestInit, "body"> & {
  body?: unknown;
  auth?: boolean;
  retry?: boolean;
};

export async function api<T>(path: string, opts: Options = {}): Promise<T> {
  const { body, auth = true, retry = true, headers, ...rest } = opts;
  const h = new Headers(headers);
  h.set("Content-Type", "application/json");
  if (auth) {
    const token = storageGet(TOKEN_KEY);
    if (token) h.set("Authorization", `Bearer ${token}`);
    const orgId = storageGet(ORG_KEY);
    if (orgId) h.set("X-Org-ID", orgId);
    const user = getStoredUser();
    if (user?.id) h.set("X-User", user.id);
  }
  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, {
      ...rest,
      headers: h,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: "include",
    });
  } catch (e) {
    throw new HttpError(0, "network_error", e instanceof Error ? e.message : "Network error");
  }
  if (res.status === 401 && auth && retry) {
    const refreshed = await tryRefresh();
    if (refreshed) return api<T>(path, { ...opts, retry: false });
  }
  const text = await res.text();
  const json: unknown = text ? safeParse(text) : null;
  if (!res.ok) {
    let code = "request_failed";
    let message = `HTTP ${res.status}`;
    if (json && typeof json === "object" && "error" in json) {
      const err: unknown = json.error;
      if (err && typeof err === "object") {
        if ("code" in err && typeof err.code === "string") code = err.code;
        if ("message" in err && typeof err.message === "string") message = err.message;
      }
    }
    throw new HttpError(res.status, code, message);
  }
  if (json && typeof json === "object" && "data" in json) return json.data as T;
  return json as T;
}

function safeParse(text: string): Record<string, unknown> | null {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

async function tryRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
      method: "POST",
      credentials: "include",
    });
    if (!res.ok) return false;
    const json: unknown = safeParse(await res.text());
    let payload: unknown = json;
    if (json && typeof json === "object" && "data" in json) payload = json.data;
    if (payload && typeof payload === "object") {
      let token: unknown = null;
      if ("access_token" in payload) token = payload.access_token;
      else if ("token" in payload) token = payload.token;
      if (typeof token === "string" && token && typeof window !== "undefined") {
        window.localStorage.setItem(TOKEN_KEY, token);
        return true;
      }
    }
    // Cookie-only session refresh still counts as success.
    return res.ok;
  } catch {
    return false;
  }
}

// ---- Domain types ----

export type Org = { id: string; name: string };
export type User = { id: string; email: string; name: string; role: string; orgs?: Org[] };
export type Category = { id: string; name: string; slug: string };
export type Product = {
  id: string;
  name: string;
  slug: string;
  description: string;
  price_cents: number;
  stock_qty: number;
  category_id: string | null;
  category?: string | Category | null;
  image_url: string | null;
  active: boolean;
};
export type Address = {
  id: string;
  street: string;
  city: string;
  state: string;
  zip: string;
  country: string;
};
export type OrderItem = {
  product_id: string;
  product_name: string;
  qty: number;
  unit_price_cents: number;
};
export type Order = {
  id: string;
  status: string;
  subtotal_cents: number;
  discount_cents: number;
  total_cents: number;
  coupon_code: string | null;
  items: OrderItem[];
  created_at: string;
};
export type CouponPreview = {
  code: string;
  type: string;
  value: number;
  min_subtotal_cents: number;
  active: boolean;
  discount_cents?: number;
};
