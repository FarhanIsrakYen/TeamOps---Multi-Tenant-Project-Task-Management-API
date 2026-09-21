import { useQueryClient } from "@tanstack/react-query";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, session } from "../lib/api";
import type { AuthTokens, Envelope, User } from "../types";

interface AuthValue {
  user: User | null;
  authenticated: boolean;
  login(email: string, password: string): Promise<void>;
  register(name: string, email: string, password: string): Promise<void>;
  logout(): Promise<void>;
}

const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [user, setUser] = useState<User | null>(
    () => session.get()?.user ?? null,
  );

  useEffect(
    () =>
      session.subscribe((value) => {
        setUser(value?.user ?? null);
        if (!value) queryClient.clear();
      }),
    [queryClient],
  );

  const apply = useCallback((tokens: AuthTokens) => session.set(tokens), []);
  const value = useMemo<AuthValue>(
    () => ({
      user,
      authenticated: Boolean(user),
      login: async (email, password) => {
        const { data } = await api.post<Envelope<AuthTokens>>("/auth/login", {
          email: email.trim().toLowerCase(),
          password,
        });
        apply(data.data);
      },
      register: async (name, email, password) => {
        const { data } = await api.post<Envelope<AuthTokens>>(
          "/auth/register",
          { name: name.trim(), email: email.trim().toLowerCase(), password },
        );
        apply(data.data);
      },
      logout: async () => {
        const active = session.get();
        try {
          if (active) {
            await api.post("/auth/logout", {
              refreshToken: active.refreshToken,
            });
          }
        } finally {
          session.clear();
        }
      },
    }),
    [apply, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth requires AuthProvider");
  return value;
}
