export type Check = {
    id: number;
    userId: number;
    url: string;
    intervalSec: number;
    lastStatus?: boolean | null;
    nextRun?: string | null;    // ISO
    updatedAt?: string | null;  // ISO
};

export type CreateCheckRequest = {
    userId: number;
    url: string;
    intervalSec: number;
};

export type CreateCheckResponse = {
    check: Check;
};

export type UpdateCheckRequest = {
    check: Check;
};

export type ListChecksResponse = {
    checks: Check[];
};
