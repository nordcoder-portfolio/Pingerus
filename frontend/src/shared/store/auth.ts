import { create } from "zustand";

export type User = {
    id: number;
    email: string;
    created_at?: string;
    updated_at?: string;
};

type AuthState = {
    accessToken: string | null;
    user: User | null;
    setAccessToken: (t: string | null) => void;
    setUser: (u: User | null) => void;
    clear: () => void;
};

const ACCESS_TOKEN_KEY = "accessToken";
const USER_KEY = "user_json";

const initialToken = localStorage.getItem(ACCESS_TOKEN_KEY);
const initialUser = (() => {
    const raw = localStorage.getItem(USER_KEY);
    if (!raw) return null;
    try {
        return JSON.parse(raw) as User;
    } catch {
        return null;
    }
})();

export const useAuthStore = create<AuthState>((set) => ({
    accessToken: initialToken,
    user: initialUser,
    setAccessToken: (t) => {
        if (t) localStorage.setItem(ACCESS_TOKEN_KEY, t);
        else localStorage.removeItem(ACCESS_TOKEN_KEY);
        set({ accessToken: t });
    },
    setUser: (u) => {
        if (u) localStorage.setItem(USER_KEY, JSON.stringify(u));
        else localStorage.removeItem(USER_KEY);
        set({ user: u });
    },
    clear: () => {
        localStorage.removeItem(ACCESS_TOKEN_KEY);
        localStorage.removeItem(USER_KEY);
        set({ accessToken: null, user: null });
    },
}));

export const authGet = () => useAuthStore.getState();
