import { createContext, useCallback, useContext, useMemo, useState } from "react";

type ToastKind = "success" | "error" | "info";
type Toast = { id: number; kind: ToastKind; title?: string; message: string; ttl?: number };

type ToastCtx = {
    push: (t: Omit<Toast, "id">) => void;
};
const Ctx = createContext<ToastCtx | null>(null);

export function ToastProvider({ children }: { children: React.ReactNode }) {
    const [items, setItems] = useState<Toast[]>([]);

    const push = useCallback((t: Omit<Toast, "id">) => {
        const toast: Toast = { id: Date.now() + Math.random(), ttl: 3500, ...t };
        setItems((prev) => [...prev, toast]);
        setTimeout(() => {
            setItems((prev) => prev.filter((x) => x.id !== toast.id));
        }, toast.ttl);
    }, []);

    const api = useMemo(() => ({ push }), [push]);

    return (
        <Ctx.Provider value={api}>
            {children}
            <div className="fixed bottom-4 right-4 z-[60] flex w-[min(92vw,380px)] flex-col gap-2">
                {items.map((t) => (
                    <div
                        key={t.id}
                        className={`card border-l-4 p-3 text-sm shadow-md ${
                            t.kind === "success"
                                ? "border-emerald-500"
                                : t.kind === "error"
                                    ? "border-rose-500"
                                    : "border-zinc-400"
                        }`}
                    >
                        {t.title && <div className="mb-1 font-semibold">{t.title}</div>}
                        <div className="text-zinc-600 dark:text-zinc-300">{t.message}</div>
                    </div>
                ))}
            </div>
        </Ctx.Provider>
    );
}

export function useToast() {
    const ctx = useContext(Ctx);
    if (!ctx) throw new Error("useToast must be used inside <ToastProvider>");
    return {
        success: (message: string, title?: string) => ctx.push({ kind: "success", message, title }),
        error: (message: string, title?: string) => ctx.push({ kind: "error", message, title }),
        info: (message: string, title?: string) => ctx.push({ kind: "info", message, title }),
    };
}
