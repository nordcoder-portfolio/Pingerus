import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useEffect, useState } from "react";
import { useAuthStore } from "../shared/store/auth";
import { logout } from "../entities/auth/api";
import { ToastProvider } from "../shared/ui/Toast";

export function AppLayout() {
    const navigate = useNavigate();

    const [theme, setTheme] = useState(
        document.documentElement.classList.contains("dark") ? "dark" : "light"
    );
    useEffect(() => {
        const saved = localStorage.getItem("theme") || "dark";
        document.documentElement.classList.toggle("dark", saved === "dark");
        setTheme(saved);
    }, []);
    const toggleTheme = () => {
        const next = theme === "dark" ? "light" : "dark";
        setTheme(next);
        document.documentElement.classList.toggle("dark", next === "dark");
        localStorage.setItem("theme", next);
    };

    const accessToken = useAuthStore((s) => s.accessToken);
    const user = useAuthStore((s) => s.user);
    const hasToken = !!accessToken;

    const handleLogout = async () => {
        await logout();
        navigate("/sign-in");
    };

    return (
        <ToastProvider>
            <div className="min-h-screen">
                <header className="sticky top-0 z-20 border-b border-zinc-200 bg-white/70 backdrop-blur dark:border-zinc-800 dark:bg-zinc-900/60">
                    <div className="container flex h-16 items-center justify-between">
                        <Link to="/" className="font-semibold tracking-tight">Pingerus</Link>

                        <nav className="flex items-center gap-2 text-sm">
                            <NavLink
                                to="/checks"
                                className={({ isActive }) =>
                                    `btn btn-ghost ${isActive ? "underline" : ""}`
                                }
                            >
                                Checks
                            </NavLink>

                            {!hasToken ? (
                                <>
                                    <NavLink to="/sign-in" className="btn btn-ghost">Sign In</NavLink>
                                    <NavLink to="/sign-up" className="btn btn-primary">Sign Up</NavLink>
                                </>
                            ) : (
                                <>
                                    {user?.email && (
                                        <span className="hidden sm:inline text-zinc-500 mr-1">{user.email}</span>
                                    )}
                                    <button onClick={handleLogout} className="btn btn-ghost">Logout</button>
                                </>
                            )}

                            <button onClick={toggleTheme} className="btn btn-ghost">Theme</button>
                        </nav>
                    </div>
                </header>

                <main className="container py-6">
                    <Outlet />
                </main>

                <footer className="border-t border-zinc-200 py-6 text-center text-xs text-zinc-500 dark:border-zinc-800">
                    © {new Date().getFullYear()} Pingerus Demo
                </footer>
            </div>
        </ToastProvider>
    );
}
