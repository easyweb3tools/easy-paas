export type ApiMeta = {
  limit?: number;
  offset?: number;
  total?: number;
  has_next?: boolean;
};

export type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
  meta?: ApiMeta;
};

export const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

const TOKEN_STORAGE_KEY = "easyweb3.auth_token";

export function apiUrl(path: string) {
  if (API_BASE) return `${API_BASE}${path}`;
  return path;
}

export function getAuthToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(TOKEN_STORAGE_KEY) ?? "";
}

export function setAuthToken(token: string) {
  if (typeof window === "undefined") return;
  const v = token.trim();
  if (!v) {
    window.localStorage.removeItem(TOKEN_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(TOKEN_STORAGE_KEY, v);
}

function withAuth(headers: HeadersInit | undefined): HeadersInit {
  // If caller explicitly sets Authorization, do not override.
  const token = getAuthToken();
  if (!token) return headers ?? {};

  const h = new Headers(headers ?? {});
  if (!h.get("Authorization")) {
    h.set("Authorization", `Bearer ${token}`);
  }
  return h;
}

export async function apiGet<T>(path: string, init?: RequestInit): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    ...init,
    headers: {
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} ${res.statusText}`);
  }
  const body = (await res.json()) as ApiResponse<T>;
  if (body.code !== 0) {
    throw new Error(body.message || "api error");
  }
  return body;
}

export async function apiPost<T>(
  path: string,
  payload?: unknown,
  init?: RequestInit
): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    method: "POST",
    ...init,
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
  }
  const body = (await res.json()) as ApiResponse<T>;
  if (body.code !== 0) {
    throw new Error(body.message || "api error");
  }
  return body;
}

export async function apiPut<T>(
  path: string,
  payload?: unknown,
  init?: RequestInit
): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    method: "PUT",
    ...init,
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
  }
  const body = (await res.json()) as ApiResponse<T>;
  if (body.code !== 0) {
    throw new Error(body.message || "api error");
  }
  return body;
}

export async function apiDelete<T>(path: string, init?: RequestInit): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    method: "DELETE",
    ...init,
    headers: {
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
  }
  const body = (await res.json()) as ApiResponse<T>;
  if (body.code !== 0) {
    throw new Error(body.message || "api error");
  }
  return body;
}
