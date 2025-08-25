import { authGet, useAuthStore } from "../store/auth";

const RUNTIME_API_BASE = window.__APP_CONFIG__?.API_BASE;
export const API_BASE =
    (RUNTIME_API_BASE && RUNTIME_API_BASE !== "${API_BASE}" ? RUNTIME_API_BASE : "") ||
    (import.meta.env.VITE_API_BASE as string) ||
    "";

type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

type FetchOptions = {
    method?: HttpMethod;
    body?: unknown;
    auth?: boolean; // по умолчанию true
    signal?: AbortSignal;
    headers?: Record<string, string>;
};

async function refreshAccessTokenOnce(): Promise<string | null> {
    try {
        const res = await fetch(`${API_BASE}/v1/auth/refresh`, {
            method: "POST",
            credentials: "include",
            headers: { "Content-Type": "application/json" },
        });
        if (!res.ok) return null;
        const data = (await res.json()) as { access_token?: string; accessToken?: string };
        const token = data.access_token ?? data.accessToken ?? null;
        useAuthStore.getState().setAccessToken(token);
        return token;
    } catch {
        return null;
    }
}

export async function fetchJson<T>(
    path: string,
    opts: FetchOptions = {}
): Promise<T> {
    const {
        method = "GET",
        body,
        auth = true,
        signal,
        headers: extraHeaders = {},
    } = opts;

    const headers: Record<string, string> = {
        "Content-Type": "application/json",
        ...extraHeaders,
    };

    const token = auth ? authGet().accessToken : null;
    if (auth && token) {
        headers.Authorization = `Bearer ${token}`;
    }

    const makeRequest = () =>
        fetch(`${API_BASE}${path}`, {
            method,
            credentials: "include",
            headers,
            body: body ? JSON.stringify(body) : undefined,
            signal,
        });

    let res = await makeRequest();

    if (auth && res.status === 401) {
        const newToken = await refreshAccessTokenOnce();
        if (newToken) {
            headers.Authorization = `Bearer ${newToken}`;
            res = await makeRequest();
        } else {
            useAuthStore.getState().clear();
        }
    }

    if (!res.ok) {
        const text = await res.text().catch(() => "");
        throw new Error(text || `HTTP ${res.status}`);
    }

    const ct = res.headers.get("content-type") || "";
    if (ct.includes("application/json")) {
        return (await res.json()) as T;
    }
    // @ts-expect-error — сервер может вернуть пустое тело
    return undefined;
}
