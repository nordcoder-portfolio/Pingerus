import { fetchJson } from "../../shared/api/client";
import type {
    SignInRequest,
    SignUpRequest,
    AuthResponse,
} from "./types";
import { useAuthStore } from "../../shared/store/auth";
import type { User } from "../../shared/store/auth";

export async function signIn(payload: SignInRequest): Promise<AuthResponse> {
    const data = await fetchJson<AuthResponse>("/v1/auth/sign-in", {
        method: "POST",
        body: payload,
        auth: false,
    });
    useAuthStore.getState().setAccessToken(data.accessToken);
    useAuthStore.getState().setUser(data.user);
    return data;
}

export async function signUp(payload: SignUpRequest): Promise<AuthResponse> {
    const data = await fetchJson<AuthResponse>("/v1/auth/sign-up", {
        method: "POST",
        body: payload,
        auth: false,
    });
    useAuthStore.getState().setAccessToken(data.accessToken);
    useAuthStore.getState().setUser(data.user);
    return data;
}

export async function logout(): Promise<void> {
    try {
        await fetchJson<void>("/v1/auth/logout", { method: "POST" });
    } finally {
        useAuthStore.getState().clear();
    }
}

export async function me(): Promise<User> {
    const user = await fetchJson<User>("/v1/auth/me", { method: "GET" });
    useAuthStore.getState().setUser(user);
    return user;
}
