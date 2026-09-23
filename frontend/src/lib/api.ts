import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios";
import type { ApiErrorBody, AuthTokens, Envelope } from "../types";

const apiBaseURL =
  (typeof window !== "undefined"
    ? window.__TEAMOPS_CONFIG__?.apiBaseUrl
    : undefined) ||
  import.meta.env.VITE_API_URL ||
  "http://localhost:8080/api/v1";

type SessionListener = (value: AuthTokens | null) => void;
let currentSession: AuthTokens | null = null;
let sessionRevision = 0;
const listeners = new Set<SessionListener>();

interface SessionSnapshot {
  revision: number;
  value: AuthTokens | null;
}

export const session = {
  get: () => currentSession,
  set(value: AuthTokens) {
    currentSession = value;
    sessionRevision += 1;
    listeners.forEach((listener) => listener(value));
  },
  clear() {
    currentSession = null;
    sessionRevision += 1;
    listeners.forEach((listener) => listener(null));
  },
  snapshot(): SessionSnapshot {
    return { revision: sessionRevision, value: currentSession };
  },
  replaceIfCurrent(snapshot: SessionSnapshot, value: AuthTokens): boolean {
    if (snapshot.revision !== sessionRevision) return false;
    session.set(value);
    return true;
  },
  clearIfCurrent(snapshot: SessionSnapshot): boolean {
    if (snapshot.revision !== sessionRevision) return false;
    session.clear();
    return true;
  },
  subscribe(listener: SessionListener) {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },
};

export const api = axios.create({ baseURL: apiBaseURL, timeout: 15_000 });
const refreshClient = axios.create({ baseURL: apiBaseURL, timeout: 15_000 });
let refreshInFlight: Promise<string> | null = null;

async function refreshAccessToken(): Promise<string> {
  const snapshot = session.snapshot();
  const active = snapshot.value;
  if (!active) throw new Error("No active session");
  refreshInFlight ??= refreshClient
    .post<Envelope<AuthTokens>>("/auth/refresh", {
      refreshToken: active.refreshToken,
    })
    .then(({ data }) => {
      if (!session.replaceIfCurrent(snapshot, data.data)) {
        throw new Error("Session changed while token refresh was in progress");
      }
      return data.data.accessToken;
    })
    .catch((error: unknown) => {
      session.clearIfCurrent(snapshot);
      throw error;
    })
    .finally(() => {
      refreshInFlight = null;
    });
  return refreshInFlight;
}

function expiresSoon(value: AuthTokens): boolean {
  return new Date(value.accessTokenExpiresAt).getTime() <= Date.now() + 30_000;
}

api.interceptors.request.use(async (config) => {
  if (config.url?.startsWith("/auth/")) return config;
  const active = session.get();
  if (!active) return config;
  const token = expiresSoon(active)
    ? await refreshAccessToken()
    : active.accessToken;
  config.headers.Authorization = `Bearer ${token}`;
  return config;
});

api.interceptors.response.use(undefined, async (error: AxiosError) => {
  const request = error.config as
    (InternalAxiosRequestConfig & { _retried?: boolean }) | undefined;
  if (
    error.response?.status !== 401 ||
    !request ||
    request._retried ||
    request.url?.startsWith("/auth/") ||
    !session.get()
  ) {
    return Promise.reject(error);
  }
  request._retried = true;
  try {
    request.headers.Authorization = `Bearer ${await refreshAccessToken()}`;
    return api(request);
  } catch (refreshError) {
    return Promise.reject(refreshError);
  }
});

export function apiMessage(error: unknown): string {
  if (axios.isAxiosError<{ error?: ApiErrorBody }>(error)) {
    return error.response?.data?.error?.message ?? error.message;
  }
  return "Something went wrong";
}

export function apiRequestID(error: unknown): string | undefined {
  if (axios.isAxiosError<{ error?: ApiErrorBody }>(error)) {
    return error.response?.data?.error?.requestId;
  }
  return undefined;
}

export function queryString(values: Record<string, unknown>): string {
  const params = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => {
    if (value !== "" && value !== undefined && value !== null) {
      params.set(key, String(value));
    }
  });
  const encoded = params.toString();
  return encoded ? `?${encoded}` : "";
}
