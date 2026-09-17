import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, session } from "../lib/api";
import type { AuthTokens, User } from "../types";

interface AuthValue {
  user: User | null;
  authenticated: boolean;
  login(email: string, password: string): Promise<void>;
  register(name: string, email: string, password: string): Promise<void>;
  logout(): Promise<void>;
}
const AuthContext = createContext<AuthValue | null>(null);
export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(
    () => session.get()?.user ?? null,
  );
  useEffect(() => {
    const clear = () => setUser(null);
    window.addEventListener("teamops:logout", clear);
    return () => window.removeEventListener("teamops:logout", clear);
  }, []);
  const apply = (tokens: AuthTokens) => {
    session.set(tokens);
    setUser(tokens.user);
  };
  const value = useMemo<AuthValue>(
    () => ({
      user,
      authenticated: Boolean(user),
      login: async (email, password) => {
        const { data } = await api.post<{ data: AuthTokens }>("/auth/login", {
          email,
          password,
        });
        apply(data.data);
      },
      register: async (name, email, password) => {
        const { data } = await api.post<{ data: AuthTokens }>(
          "/auth/register",
          { name, email, password },
        );
        apply(data.data);
      },
      logout: async () => {
        const current = session.get();
        try {
          if (current)
            await api.post("/auth/logout", {
              refreshToken: current.refreshToken,
            });
        } finally {
          session.clear();
          setUser(null);
        }
      },
    }),
    [user],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth requires AuthProvider");
  return value;
}
