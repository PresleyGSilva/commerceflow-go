"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api, ORG_KEY, TOKEN_KEY, USER_KEY, type Org, type User } from "./api";

type AuthState = {
  user: User | null;
  orgs: Org[];
  orgId: string | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => void;
  setOrgId: (id: string) => void;
};

const AuthContext = createContext<AuthState | null>(null);

function persistSession(token: string, user: User, orgs: Org[]) {
  window.localStorage.setItem(TOKEN_KEY, token);
  window.localStorage.setItem(USER_KEY, JSON.stringify(user));
  // Login MUST persist orgs[0].id so tenant headers work store-wide.
  const first = orgs[0]?.id ?? null;
  if (first) window.localStorage.setItem(ORG_KEY, first);
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [orgs, setOrgs] = useState<Org[]>([]);
  const [orgId, setOrgIdState] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    try {
      const rawUser = window.localStorage.getItem(USER_KEY);
      const rawOrgs = window.localStorage.getItem("commerceflow_orgs");
      const rawOrg = window.localStorage.getItem(ORG_KEY);
      if (rawUser) setUser(JSON.parse(rawUser));
      if (rawOrgs) setOrgs(JSON.parse(rawOrgs));
      if (rawOrg) setOrgIdState(rawOrg);
    } catch {
      // corrupted storage: start clean
    } finally {
      setLoading(false);
    }
  }, []);

  const applySession = useCallback((token: string, u: User, o: Org[]) => {
    persistSession(token, u, o);
    window.localStorage.setItem("commerceflow_orgs", JSON.stringify(o));
    setUser(u);
    setOrgs(o);
    setOrgIdState(o[0]?.id ?? null);
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const data = await api<{
        access_token?: string;
        token?: string;
        user: User;
        orgs?: Org[];
      }>("/api/v1/auth/login", { method: "POST", body: { email, password }, auth: false });
      const token = data.access_token ?? data.token ?? "";
      const o = data.orgs ?? data.user.orgs ?? [];
      if (token) applySession(token, data.user, o);
      else {
        // cookie-session fallback
        window.localStorage.setItem(USER_KEY, JSON.stringify(data.user));
        window.localStorage.setItem("commerceflow_orgs", JSON.stringify(o));
        setUser(data.user);
        setOrgs(o);
        if (o[0]?.id) {
          window.localStorage.setItem(ORG_KEY, o[0].id);
          setOrgIdState(o[0].id);
        }
      }
    },
    [applySession],
  );

  const register = useCallback(
    async (name: string, email: string, password: string) => {
      const data = await api<{
        access_token?: string;
        token?: string;
        user: User;
        orgs?: Org[];
      }>("/api/v1/auth/register", {
        method: "POST",
        body: { name, email, password },
        auth: false,
      });
      const token = data.access_token ?? data.token ?? "";
      const o = data.orgs ?? data.user.orgs ?? [];
      if (token || data.user) {
        if (token) applySession(token, data.user, o);
        else {
          window.localStorage.setItem(USER_KEY, JSON.stringify(data.user));
          setUser(data.user);
        }
      }
    },
    [applySession],
  );

  const logout = useCallback(() => {
    window.localStorage.removeItem(TOKEN_KEY);
    window.localStorage.removeItem(USER_KEY);
    window.localStorage.removeItem(ORG_KEY);
    window.localStorage.removeItem("commerceflow_orgs");
    setUser(null);
    setOrgs([]);
    setOrgIdState(null);
    api("/api/v1/auth/logout", { method: "POST" }).catch(() => undefined);
  }, []);

  const setOrgId = useCallback((id: string) => {
    window.localStorage.setItem(ORG_KEY, id);
    setOrgIdState(id);
  }, []);

  const value = useMemo(
    () => ({ user, orgs, orgId, loading, login, register, logout, setOrgId }),
    [user, orgs, orgId, loading, login, register, logout, setOrgId],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
