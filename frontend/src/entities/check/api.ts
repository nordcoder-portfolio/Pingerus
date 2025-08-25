import { fetchJson } from "../../shared/api/client";
import type {
    Check,
    CreateCheckRequest,
    CreateCheckResponse,
    UpdateCheckRequest,
    ListChecksResponse,
} from "./types";

// Нормализуем и camelCase, и snake_case на всякий случай
function toCheck(raw: any): Check {
    return {
        id: Number(raw.id),
        userId: Number(raw.userId ?? raw.user_id),
        url: String(raw.url),
        intervalSec: Number(raw.intervalSec ?? raw.interval_sec),
        lastStatus: raw.lastStatus ?? raw.last_status ?? null,
        nextRun: (raw.nextRun ?? raw.next_run) ?? null,
        updatedAt: (raw.updatedAt ?? raw.updated_at) ?? null,
    };
}

export async function listChecks(userId: number): Promise<Check[]> {
    const data = await fetchJson<{ checks: any[] }>(`/v1/users/${userId}/checks`, { method: "GET" });
    const arr = Array.isArray(data?.checks) ? data.checks : [];
    return arr.map(toCheck);
}

export async function getCheck(id: number): Promise<Check> {
    const raw = await fetchJson<any>(`/v1/checks/${id}`, { method: "GET" });
    return toCheck(raw);
}

export async function createCheck(payload: CreateCheckRequest): Promise<Check> {
    // Отправляем camelCase (grpc-gateway его и ждёт)
    const body = {
        userId: payload.userId,
        url: payload.url,
        intervalSec: payload.intervalSec,
    };
    const data = await fetchJson<CreateCheckResponse>("/v1/checks", {
        method: "POST",
        body,
    });
    return toCheck(data.check);
}

export async function updateCheck(req: UpdateCheckRequest): Promise<Check> {
    const c = req.check;
    const body = {
        check: {
            id: c.id,
            userId: c.userId,
            url: c.url,
            intervalSec: c.intervalSec,
            lastStatus: c.lastStatus ?? undefined,
            nextRun: c.nextRun ?? undefined,
            updatedAt: c.updatedAt ?? undefined,
        },
    };
    const raw = await fetchJson<any>(`/v1/checks/${c.id}`, {
        method: "PUT",
        body,
    });
    return toCheck(raw);
}

export async function deleteCheck(id: number): Promise<void> {
    await fetchJson<void>(`/v1/checks/${id}`, { method: "DELETE" });
}
