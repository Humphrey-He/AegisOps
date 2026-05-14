import { API_BASE_URL } from "./config";
import { ApiError, type ApiResponse } from "../types/api";
import { useSessionStore } from "../store/sessionStore";

type RequestOptions = RequestInit & {
  body?: unknown;
};

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const token = useSessionStore.getState().token;
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.headers ?? {}),
    },
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });

  let payload: ApiResponse<T> | null = null;
  try {
    payload = (await response.json()) as ApiResponse<T>;
  } catch {
    payload = null;
  }

  if (!response.ok || !payload) {
    const traceId = payload?.traceId ?? crypto.randomUUID();
    const message = payload?.message ?? `请求失败 (${response.status})`;
    if (response.status === 401) {
      useSessionStore.getState().clearSession();
    }
    throw new ApiError({
      status: response.status,
      code: payload?.code ?? "HTTP_ERROR",
      message,
      traceId,
    });
  }

  return payload.data;
}

export const http = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: "POST", body }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: "PATCH", body }),
};
