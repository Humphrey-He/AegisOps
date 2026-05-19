import { API_BASE_URL } from "./config";
import { ApiError, type ApiResponse } from "../types/api";
import { useSessionStore } from "../store/sessionStore";

type RequestOptions = Omit<RequestInit, "body"> & {
  body?: unknown;
};

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const token = useSessionStore.getState().token;
  const { body, headers, ...rest } = options;
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...rest,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(headers ?? {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
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

type DownloadedFile = {
  blob: Blob;
  fileName?: string;
  contentType?: string;
};

async function download(path: string, options: Omit<RequestOptions, "body"> = {}): Promise<DownloadedFile> {
  const token = useSessionStore.getState().token;
  const { headers, ...rest } = options;
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...rest,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(headers ?? {}),
    },
  });

  if (!response.ok) {
    let payload: ApiResponse<unknown> | null = null;
    try {
      payload = (await response.json()) as ApiResponse<unknown>;
    } catch {
      payload = null;
    }

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

  const contentDisposition = response.headers.get("content-disposition") ?? "";
  const fileNameMatch = contentDisposition.match(/filename\*?=(?:UTF-8''|")?([^\";]+)/i);
  const fileName = fileNameMatch?.[1] ? decodeURIComponent(fileNameMatch[1].replace(/"/g, "").trim()) : undefined;

  return {
    blob: await response.blob(),
    fileName,
    contentType: response.headers.get("content-type") ?? undefined,
  };
}

export const http = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: "POST", body }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: "PATCH", body }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  download: (path: string) => download(path),
};
