import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios";
import type { ApiErrorBody, AuthTokens, Envelope } from "../types";

const apiBaseURL =
  import.meta.env.VITE_API_URL ?? "http://localhost:8080/api/v1";

type SessionListener = (value: AuthTokens | null) => void;
let currentSession: AuthTokens | null = null;
const listeners = new Set<SessionListener>();

export const session = {
  get: () => currentSession,
  set(value: AuthTokens) {
    currentSession = value;
    listeners.forEach((listener) => listener(value));
  },
  clear() {
    currentSession = null;
    listeners.forEach((listener) => listener(null));
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
  const active = session.get();
  if (!active) throw new Error("No active session");
  refreshInFlight ??= refreshClient
    .post<Envelope<AuthTokens>>("/auth/refresh", {
      refreshToken: active.refreshToken,
    })
    .then(({ data }) => {
      session.set(data.data);
      return data.data.accessToken;
    })
    .catch((error: unknown) => {
      session.clear();
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
