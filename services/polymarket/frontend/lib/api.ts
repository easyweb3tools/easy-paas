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
const LEGACY_TOKEN_AUTH = process.env.NEXT_PUBLIC_LEGACY_TOKEN_AUTH === "1";

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

function normalizeRequestError(status: number): string {
  if (status === 401) return "Unauthorized";
  if (status === 403) return "Forbidden";
  if (status >= 500) return "Server error, please retry later";
  return "Request failed, please try again";
}

async function parseApiResponse<T>(res: Response): Promise<ApiResponse<T>> {
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    if (typeof window !== "undefined") {
      console.error(`API error: ${res.status} ${res.statusText}`, text);
    }
    throw new Error(normalizeRequestError(res.status));
  }
  const body = (await res.json()) as ApiResponse<T>;
  if (body.code !== 0) {
    throw new Error(body.message || "Request failed, please try again");
  }
  return body;
}

function withAuth(headers: HeadersInit | undefined): HeadersInit {
  // If caller explicitly sets Authorization, do not override.
  if (!LEGACY_TOKEN_AUTH) return headers ?? {};
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
    credentials: init?.credentials ?? "include",
    headers: {
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
  });
  return parseApiResponse<T>(res);
}

export async function apiPost<T>(
  path: string,
  payload?: unknown,
  init?: RequestInit
): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    method: "POST",
    ...init,
    credentials: init?.credentials ?? "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  return parseApiResponse<T>(res);
}

export async function apiPut<T>(
  path: string,
  payload?: unknown,
  init?: RequestInit
): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    method: "PUT",
    ...init,
    credentials: init?.credentials ?? "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  return parseApiResponse<T>(res);
}

export async function apiDelete<T>(path: string, init?: RequestInit): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    method: "DELETE",
    ...init,
    credentials: init?.credentials ?? "include",
    headers: {
      Accept: "application/json",
      ...withAuth(init?.headers),
    },
  });
  return parseApiResponse<T>(res);
}
