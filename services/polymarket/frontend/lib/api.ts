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

export function apiUrl(path: string) {
  if (API_BASE) return `${API_BASE}${path}`;
  return path;
}

export async function apiGet<T>(path: string, init?: RequestInit): Promise<ApiResponse<T>> {
  const res = await fetch(apiUrl(path), {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.headers ?? {}),
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
      ...(init?.headers ?? {}),
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
      ...(init?.headers ?? {}),
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
      ...(init?.headers ?? {}),
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
