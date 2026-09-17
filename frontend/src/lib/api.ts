import axios from "axios";
import type { AuthTokens } from "../types";

const storageKey = "teamops.session";
export const session = {
  get: (): AuthTokens | null => {
    try {
      return JSON.parse(
        localStorage.getItem(storageKey) ?? "null",
      ) as AuthTokens | null;
    } catch {
      return null;
    }
  },
  set: (value: AuthTokens) =>
    localStorage.setItem(storageKey, JSON.stringify(value)),
  clear: () => localStorage.removeItem(storageKey),
};

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? "http://localhost:8080/api/v1",
  timeout: 15_000,
});
api.interceptors.request.use((config) => {
  const token = session.get()?.accessToken;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

let refreshInFlight: Promise<string> | null = null;
api.interceptors.response.use(undefined, async (error) => {
  const request = error.config as typeof error.config & { _retried?: boolean };
  if (
    error.response?.status !== 401 ||
    request?._retried ||
    request?.url?.includes("/auth/refresh")
  )
    return Promise.reject(error);
  const current = session.get();
  if (!current) return Promise.reject(error);
  request._retried = true;
  refreshInFlight ??= axios
    .post<{ data: AuthTokens }>(`${api.defaults.baseURL}/auth/refresh`, {
      refreshToken: current.refreshToken,
    })
    .then(({ data }) => {
      session.set(data.data);
      return data.data.accessToken;
    })
    .finally(() => {
      refreshInFlight = null;
    });
  try {
    const token = await refreshInFlight;
    request.headers.Authorization = `Bearer ${token}`;
    return api(request);
  } catch (refreshError) {
    session.clear();
    window.dispatchEvent(new Event("teamops:logout"));
    return Promise.reject(refreshError);
  }
});

export function apiMessage(error: unknown): string {
  if (axios.isAxiosError(error))
    return error.response?.data?.error?.message ?? error.message;
  return "Something went wrong";
}
