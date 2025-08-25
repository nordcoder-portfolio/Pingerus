import React, { useEffect, useMemo, useState } from "react";

const API_BASE =
    (typeof window !== "undefined" &&
        window.__APP_CONFIG__ &&
        window.__APP_CONFIG__.API_BASE) ||
    (typeof import.meta !== "undefined" &&
        import.meta.env &&
        import.meta.env.VITE_API_BASE) ||
    "";

function apiUrl(path) {
    console.log(API_BASE)
    return `${API_BASE}${path}`;
}

function useLocalStorage(key, initial) {
    const [value, setValue] = useState(() => {
        try {
            const raw = window.localStorage.getItem(key);
            return raw ? JSON.parse(raw) : initial;
        } catch {
            return initial;
        }
    });
    useEffect(() => {
        try {
            window.localStorage.setItem(key, JSON.stringify(value));
        } catch {}
    }, [key, value]);
    return [value, setValue];
}

async function http(path, { method = "GET", body, token } = {}) {
    const headers = { "Content-Type": "application/json" };
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const res = await fetch(apiUrl(path), {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
        credentials: "include",
    });
    const text = await res.text();
    let data;
    try { data = text ? JSON.parse(text) : null; } catch { data = text; }
    if (!res.ok) {
        const msg = (data && (data.error || data.message)) || res.statusText;
        throw new Error(msg || `HTTP ${res.status}`);
    }
    return data;
}

function Badge({ children, kind = "neutral" }) {
    const map = {
        neutral: "bg-gray-200 text-gray-800",
        green: "bg-green-200 text-green-900",
        red: "bg-red-200 text-red-900",
        blue: "bg-blue-200 text-blue-900",
        amber: "bg-amber-200 text-amber-900",
    };
    return (
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${map[kind]}`}>{children}</span>
    );
}

function Card({ title, actions, children }) {
    return (
        <div className="rounded-2xl border border-gray-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b p-4">
                <h3 className="text-lg font-semibold">{title}</h3>
                <div className="flex gap-2">{actions}</div>
            </div>
            <div className="p-4">{children}</div>
        </div>
    );
}

function TextInput({ label, type = "text", value, onChange, placeholder, disabled, className = "" }) {
    return (
        <label className={`block ${className}`}>
            <span className="mb-1 block text-sm text-gray-700">{label}</span>
            <input
                type={type}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                placeholder={placeholder}
                disabled={disabled}
                className="w-full rounded-xl border border-gray-300 px-3 py-2 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-200 disabled:opacity-60"
            />
        </label>
    );
}

function Button({ children, onClick, type = "button", variant = "primary", disabled }) {
    const base = "inline-flex items-center justify-center rounded-xl px-4 py-2 text-sm font-medium shadow-sm transition focus:outline-none focus:ring-2 focus:ring-offset-2";
    const variants = {
        primary: "bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500",
        ghost: "bg-transparent text-gray-700 hover:bg-gray-100 focus:ring-gray-300",
        danger: "bg-red-600 text-white hover:bg-red-700 focus:ring-red-500",
        outline: "border border-gray-300 bg-white text-gray-800 hover:bg-gray-50 focus:ring-gray-300",
    };
    return (
        <button type={type} onClick={onClick} disabled={disabled} className={`${base} ${variants[variant]} disabled:opacity-60`}>
            {children}
        </button>
    );
}

function Divider() {
    return <div className="my-4 h-px w-full bg-gray-200" />;
}

function prettyTs(ts) {
    if (!ts) return "—";
    try {
        if (typeof ts === "string") return new Date(ts).toLocaleString();
        if (typeof ts === "object" && ts.seconds != null) {
            const ms = (Number(ts.seconds) * 1000) + Math.floor(Number(ts.nanos || 0) / 1e6);
            return new Date(ms).toLocaleString();
        }
    } catch {}
    return String(ts);
}

export default function PingerusDemo() {
    const [token, setToken] = useLocalStorage("pingerus_token", "");
    const [user, setUser] = useState(null);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");

    const [mode, setMode] = useState("sign-in");
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");

    const [checks, setChecks] = useState([]);
    const [form, setForm] = useState({ url: "https://example.com", interval_sec: 60 });
    const [editing, setEditing] = useState(null); // check being edited

    const authed = !!token;

    useEffect(() => {
        if (!authed) return;
        refreshMe().catch(() => {});
    }, [authed]);

    async function refreshMe() {
        try {
            setError("");
            const me = await http("/v1/auth/me", { token });
            setUser(me);
            if (me?.id) await loadChecks(me.id);
        } catch (e) {
            setUser(null);
            setError(e.message);
        }
    }

    async function onSignIn(e) {
        e?.preventDefault();
        try {
            setBusy(true); setError("");
            const data = await http("/v1/auth/sign-in", { method: "POST", body: { email, password } });
            setToken(data?.accessToken || "");
            setUser(data?.user || null);
            if (data?.user?.id) await loadChecks(data.user.id);
        } catch (e) {
            setError(e.message);
        } finally { setBusy(false); }
    }

    async function onSignUp(e) {
        e?.preventDefault();
        try {
            setBusy(true); setError("");
            const data = await http("/v1/auth/sign-up", { method: "POST", body: { email, password } });
            setToken(data?.access_token || "");
            setUser(data?.user || null);
            if (data?.user?.id) await loadChecks(data.user.id);
        } catch (e) {
            setError(e.message);
        } finally { setBusy(false); }
    }

    async function onRefreshToken() {
        try {
            setBusy(true); setError("");
            const data = await http("/v1/auth/refresh", { method: "POST", token });
            setToken(data?.access_token || "");
        } catch (e) {
            setError(e.message);
        } finally { setBusy(false); }
    }

    async function onLogout() {
        try {
            setBusy(true); setError("");
            await http("/v1/auth/logout", { method: "POST", token });
        } catch (e) {
            // ignore backend errors on demo logout
        } finally {
            setToken(""); setUser(null); setChecks([]); setBusy(false);
        }
    }

    async function loadChecks(userId) {
        const data = await http(`/v1/users/${userId}/checks`, { token });
        setChecks(data?.checks || []);
    }

    async function createCheck(e) {
        e?.preventDefault();
        if (!user?.id) return;
        try {
            setBusy(true); setError("");
            const data = await http("/v1/checks", { method: "POST", token, body: { user_id: user.id, url: form.url, interval_sec: Number(form.interval_sec) || 60 } });
            if (data?.check) setChecks((prev) => [data.check, ...prev]);
            setForm({ url: "https://example.com", interval_sec: 60 });
        } catch (e) { setError(e.message); }
        finally { setBusy(false); }
    }

    async function updateCheck(e) {
        e?.preventDefault();
        if (!editing) return;
        try {
            setBusy(true); setError("");
            const body = { check: editing };
            const updated = await http(`/v1/checks/${editing.id}`, { method: "PUT", token, body });
            setChecks((prev) => prev.map((c) => (c.id === updated.id ? updated : c)));
            setEditing(null);
        } catch (e) { setError(e.message); }
        finally { setBusy(false); }
    }

    async function deleteCheck(id) {
        try {
            setBusy(true); setError("");
            await http(`/v1/checks/${id}`, { method: "DELETE", token });
            setChecks((prev) => prev.filter((c) => c.id !== id));
        } catch (e) { setError(e.message); }
        finally { setBusy(false); }
    }

    const header = (
        <header className="sticky top-0 z-10 border-b bg-white/80 backdrop-blur">
            <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3">
                <div className="flex items-center gap-3">
                    <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-600 text-white">P</div>
                    <div>
                        <h1 className="text-lg font-semibold">Pingerus Demo</h1>
                        <p className="text-xs text-gray-500">quick auth & checks UI</p>
                    </div>
                </div>
                <div className="flex items-center gap-3">
                    <Badge kind="blue">{API_BASE || "same-origin"}</Badge>
                    {authed ? (
                        <div className="flex items-center gap-2">
                            <Badge kind="green">signed in</Badge>
                            <Button variant="outline" onClick={onRefreshToken} disabled={busy}>Refresh token</Button>
                            <Button variant="ghost" onClick={refreshMe} disabled={busy}>Reload Me</Button>
                            <Button variant="danger" onClick={onLogout} disabled={busy}>Logout</Button>
                        </div>
                    ) : (
                        <Badge>guest</Badge>
                    )}
                </div>
            </div>
        </header>
    );

    return (
        <div className="min-h-screen bg-gray-50 text-gray-900">
            {header}
            <main className="mx-auto grid max-w-6xl grid-cols-1 gap-6 p-4 md:grid-cols-2">
                {!authed && (
                    <Card
                        title={mode === "sign-in" ? "Sign In" : "Sign Up"}
                        actions={
                            <Button variant="ghost" onClick={() => setMode(mode === "sign-in" ? "sign-up" : "sign-in")}>{mode === "sign-in" ? "Need an account?" : "Have an account?"}</Button>
                        }
                    >
                        <form onSubmit={mode === "sign-in" ? onSignIn : onSignUp} className="space-y-3">
                            <TextInput label="Email" type="email" value={email} onChange={setEmail} placeholder="you@example.com" />
                            <TextInput label="Password" type="password" value={password} onChange={setPassword} placeholder={mode === "sign-up" ? "8-128 characters" : "••••••••"} />
                            <div className="flex items-center gap-2">
                                <Button type="submit" disabled={busy}>{mode === "sign-in" ? "Sign In" : "Create account"}</Button>
                                {error && <span className="text-sm text-red-600">{error}</span>}
                            </div>
                            <p className="text-xs text-gray-500">Credentials are sent to your API: /v1/auth/{mode === "sign-in" ? "sign-in" : "sign-up"}</p>
                        </form>
                    </Card>
                )}

                {authed && (
                    <Card title="Account">
                        <div className="space-y-2 text-sm">
                            <div className="flex items-center justify-between">
                                <span className="text-gray-600">Access token:</span>
                                <code className="line-clamp-1 max-w-[60%] rounded bg-gray-100 px-2 py-1">{token || "—"}</code>
                            </div>
                            <Divider />
                            <div className="grid grid-cols-2 gap-3">
                                <div>
                                    <div className="text-xs uppercase text-gray-500">User ID</div>
                                    <div className="font-medium">{user?.id ?? "—"}</div>
                                </div>
                                <div>
                                    <div className="text-xs uppercase text-gray-500">Email</div>
                                    <div className="font-medium">{user?.email ?? "—"}</div>
                                </div>
                                <div>
                                    <div className="text-xs uppercase text-gray-500">Created</div>
                                    <div className="font-medium">{prettyTs(user?.created_at)}</div>
                                </div>
                                <div>
                                    <div className="text-xs uppercase text-gray-500">Updated</div>
                                    <div className="font-medium">{prettyTs(user?.updated_at)}</div>
                                </div>
                            </div>
                            {error && <p className="text-sm text-red-600">{error}</p>}
                        </div>
                    </Card>
                )}

                {authed && (
                    <Card title="Create Check">
                        <form onSubmit={createCheck} className="grid grid-cols-1 gap-3 md:grid-cols-3">
                            <TextInput label="URL" value={form.url} onChange={(v) => setForm((s) => ({ ...s, url: v }))} className="md:col-span-2" />
                            <TextInput label="Interval (sec)" type="number" value={form.interval_sec} onChange={(v) => setForm((s) => ({ ...s, interval_sec: v }))} />
                            <div className="md:col-span-3 flex items-center gap-2">
                                <Button type="submit" disabled={busy}>Create</Button>
                                <span className="text-xs text-gray-500">POST /v1/checks</span>
                            </div>
                        </form>
                    </Card>
                )}

                {authed && (
                    <Card title="Your Checks" actions={<Button variant="ghost" onClick={() => loadChecks(user?.id)} disabled={busy}>Reload</Button>}>
                        {checks.length === 0 ? (
                            <p className="text-sm text-gray-600">No checks yet.</p>
                        ) : (
                            <div className="overflow-x-auto">
                                <table className="min-w-full text-sm">
                                    <thead>
                                    <tr className="text-left text-gray-600">
                                        <th className="px-2 py-2">ID</th>
                                        <th className="px-2 py-2">URL</th>
                                        <th className="px-2 py-2">Interval</th>
                                        <th className="px-2 py-2">Last Status</th>
                                        <th className="px-2 py-2">Next Run</th>
                                        <th className="px-2 py-2">Updated</th>
                                        <th className="px-2 py-2">Actions</th>
                                    </tr>
                                    </thead>
                                    <tbody>
                                    {checks.map((c) => (
                                        <tr key={c.id} className="border-t">
                                            <td className="px-2 py-2 font-mono">{c.id}</td>
                                            <td className="px-2 py-2">
                                                <a href={c.url} target="_blank" rel="noreferrer" className="text-blue-600 underline-offset-2 hover:underline">{c.url}</a>
                                            </td>
                                            <td className="px-2 py-2">{c.interval_sec}s</td>
                                            <td className="px-2 py-2">{c.last_status == null ? <Badge>—</Badge> : c.last_status ? <Badge kind="green">UP</Badge> : <Badge kind="red">DOWN</Badge>}</td>
                                            <td className="px-2 py-2">{prettyTs(c.next_run)}</td>
                                            <td className="px-2 py-2">{prettyTs(c.updated_at)}</td>
                                            <td className="px-2 py-2">
                                                <div className="flex gap-2">
                                                    <Button variant="outline" onClick={() => setEditing(c)}>Edit</Button>
                                                    <Button variant="danger" onClick={() => deleteCheck(c.id)} disabled={busy}>Delete</Button>
                                                </div>
                                            </td>
                                        </tr>
                                    ))}
                                    </tbody>
                                </table>
                            </div>
                        )}
                    </Card>
                )}

                {}
                {editing && (
                    <div className="fixed inset-0 z-20 flex items-center justify-center bg-black/40 p-4">
                        <div className="w-full max-w-xl rounded-2xl bg-white p-6 shadow-xl">
                            <div className="mb-4 flex items-center justify-between">
                                <h3 className="text-lg font-semibold">Edit Check #{editing.id}</h3>
                                <button onClick={() => setEditing(null)} className="rounded-lg p-1 hover:bg-gray-100">✕</button>
                            </div>
                            <form onSubmit={updateCheck} className="grid grid-cols-1 gap-3 md:grid-cols-3">
                                <TextInput label="URL" value={editing.url || ""} onChange={(v) => setEditing((s) => ({ ...s, url: v }))} className="md:col-span-2" />
                                <TextInput label="Interval (sec)" type="number" value={editing.interval_sec || 60} onChange={(v) => setEditing((s) => ({ ...s, interval_sec: Number(v) }))} />
                                <div className="md:col-span-3 flex items-center gap-2">
                                    <Button type="submit" disabled={busy}>Save</Button>
                                    <Button variant="ghost" onClick={() => setEditing(null)} type="button">Cancel</Button>
                                    <span className="text-xs text-gray-500">PUT /v1/checks/{editing.id}</span>
                                </div>
                            </form>
                        </div>
                    </div>
                )}
            </main>

            <footer className="mx-auto max-w-6xl px-4 py-10 text-center text-xs text-gray-500">
                <p>
                    Demo UI for Pingerus • Update API_BASE inside the file if backend runs on another host/port.
                </p>
            </footer>
        </div>
    );
}
