import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "../../shared/store/auth";
import { Modal } from "../../shared/ui/Modal";
import { Confirm } from "../../shared/ui/Confirm";
import { CheckForm } from "./CheckForm";
import type { Check } from "../../entities/check/types";
import {
    listChecks,
    createCheck,
    updateCheck,
    deleteCheck,
} from "../../entities/check/api";
import { useToast } from "../../shared/ui/Toast";

function fmtTime(iso?: string | null) {
    if (!iso) return "—";
    try {
        const d = new Date(iso);
        return new Intl.DateTimeFormat(undefined, {
            year: "numeric",
            month: "short",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
        }).format(d);
    } catch {
        return iso;
    }
}

export function ChecksPage() {
    const navigate = useNavigate();
    const { success, error, info } = useToast();

    const user = useAuthStore((s) => s.user);
    const token = useAuthStore((s) => s.accessToken);

    const [items, setItems] = useState<Check[]>([]);
    const [loading, setLoading] = useState(true);
    const [err, setErr] = useState<string | null>(null);

    const [openCreate, setOpenCreate] = useState(false);
    const [openEdit, setOpenEdit] = useState<Check | null>(null);
    const [openDelete, setOpenDelete] = useState<Check | null>(null);
    const [submitting, setSubmitting] = useState(false);

    const [query, setQuery] = useState("");

    useEffect(() => {
        if (!token) navigate("/sign-in");
    }, [token, navigate]);

    async function load() {
        if (!user?.id) return;
        setErr(null);
        try {
            const rows = await listChecks(user.id);
            setItems(rows);
        } catch (e: any) {
            const msg = e?.message || "Failed to load checks";
            setErr(msg);
            error(msg, "Load error");
        }
    }

    useEffect(() => {
        setLoading(true);
        (async () => {
            await load();
            setLoading(false);
        })();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [user?.id]);

    const filtered = useMemo(() => {
        const q = query.trim().toLowerCase();
        if (!q) return items;
        return items.filter((x) => x.url.toLowerCase().includes(q));
    }, [items, query]);

    const empty = useMemo(() => !loading && filtered.length === 0, [loading, filtered.length]);

    async function handleCreate(values: { url: string; intervalSec: number }) {
        if (!user?.id) return;
        setSubmitting(true);
        try {
            const created = await createCheck({ userId: user.id, ...values });
            setItems((prev) => [created, ...prev]);
            setOpenCreate(false);
            success("Check created");
        } catch (e: any) {
            const msg = e?.message || "Create failed";
            error(msg);
        } finally {
            setSubmitting(false);
        }
    }

    async function handleEdit(values: { url: string; intervalSec: number }) {
        if (!openEdit) return;
        setSubmitting(true);
        try {
            const updated = await updateCheck({
                check: { ...openEdit, url: values.url, intervalSec: values.intervalSec },
            });
            setItems((prev) => prev.map((x) => (x.id === updated.id ? updated : x)));
            setOpenEdit(null);
            success("Changes saved");
        } catch (e: any) {
            const msg = e?.message || "Update failed";
            error(msg);
        } finally {
            setSubmitting(false);
        }
    }

    async function handleDelete() {
        if (!openDelete) return;
        setSubmitting(true);
        const victim = openDelete;
        try {
            setItems((prev) => prev.filter((x) => x.id !== victim.id));
            setOpenDelete(null);
            await deleteCheck(victim.id);
            info(`Check #${victim.id} deleted`);
        } catch (e: any) {
            const msg = e?.message || "Delete failed";
            error(msg);
            setItems((prev) => [...prev, victim].sort((a, b) => a.id - b.id));
        } finally {
            setSubmitting(false);
        }
    }

    return (
        <div className="space-y-6">
            {/* Toolbar */}
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <h1 className="text-2xl font-semibold">Checks</h1>

                <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <div className="flex items-center gap-2">
                        <input
                            className="input"
                            placeholder="Search URL…"
                            value={query}
                            onChange={(e) => setQuery(e.target.value)}
                        />
                        <span className="text-xs text-zinc-500">({filtered.length})</span>
                    </div>

                    <button className="btn btn-ghost" onClick={load}>
                        Refresh now
                    </button>

                    <button className="btn btn-primary" onClick={() => setOpenCreate(true)}>
                        New Check
                    </button>
                </div>
            </div>

            {/* Loading / Error / Empty */}
            {loading && (
                <div className="card p-5">
                    <div className="h-4 w-48 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
                    <div className="mt-3 h-4 w-full animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
                    <div className="mt-3 h-4 w-2/3 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
                </div>
            )}

            {err && (
                <div className="rounded-xl border border-rose-300 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300">
                    {err}
                </div>
            )}

            {empty && (
                <div className="card p-6 text-center">
                    <p className="text-sm text-zinc-500">Ещё нет чеков по запросу.</p>
                    <button className="btn btn-primary mt-3" onClick={() => setOpenCreate(true)}>
                        Создать первый
                    </button>
                </div>
            )}

            {/* Grid */}
            {!loading && filtered.length > 0 && (
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                    {filtered.map((c) => (
                        <div key={c.id} className="card card-hover p-5">
                            <div className="mb-2 text-sm text-zinc-500 flex items-center justify-between">
                                <span>#{c.id}</span>
                                <span className={`font-medium ${c.lastStatus ? "text-emerald-600" : "text-rose-600"}`}>
                  {c.lastStatus === undefined || c.lastStatus === null
                      ? "—"
                      : c.lastStatus
                          ? "UP"
                          : "DOWN"}
                </span>
                            </div>

                            <div className="truncate font-mono text-sm">{c.url}</div>

                            <div className="mt-2 flex items-center justify-between text-sm">
                                <span className="text-zinc-500">Interval</span>
                                <span className="font-medium">{c.intervalSec}s</span>
                            </div>

                            <div className="mt-2 flex items-center justify-between text-xs text-zinc-500">
                                <span>Next run</span>
                                <span>{fmtTime(c.nextRun)}</span>
                            </div>
                            <div className="mt-1 flex items-center justify-between text-xs text-zinc-500">
                                <span>Updated</span>
                                <span>{fmtTime(c.updatedAt)}</span>
                            </div>

                            <div className="mt-4 flex gap-2">
                                <button
                                    className="btn btn-ghost flex-1"
                                    onClick={() => setOpenEdit(c)}
                                    disabled={submitting}
                                >
                                    {submitting && openEdit?.id === c.id ? "Saving..." : "Edit"}
                                </button>
                                <button
                                    className="btn btn-ghost flex-1"
                                    onClick={() => setOpenDelete(c)}
                                    disabled={submitting}
                                >
                                    {submitting && openDelete?.id === c.id ? "Deleting..." : "Delete"}
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Create */}
            <Modal
                open={openCreate}
                onClose={() => !submitting && setOpenCreate(false)}
                title="Create Check"
                footer={null}
            >
                <CheckForm initial={null} onSubmit={handleCreate} submitting={submitting} />
            </Modal>

            {/* Edit */}
            <Modal
                open={!!openEdit}
                onClose={() => !submitting && setOpenEdit(null)}
                title={`Edit Check #${openEdit?.id ?? ""}`}
                footer={null}
            >
                {openEdit && (
                    <CheckForm
                        initial={openEdit}
                        onSubmit={handleEdit}
                        submitting={submitting}
                    />
                )}
            </Modal>

            {/* Delete */}
            <Confirm
                open={!!openDelete}
                title={`Delete Check #${openDelete?.id ?? ""}`}
                message={`Удалить проверку "${openDelete?.url ?? ""}"?`}
                confirmText="Delete"
                cancelText="Cancel"
                loading={submitting}
                onConfirm={handleDelete}
                onCancel={() => !submitting && setOpenDelete(null)}
            />
        </div>
    );
}
