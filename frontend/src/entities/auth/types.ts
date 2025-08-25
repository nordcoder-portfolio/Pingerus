import type { User } from "@shared/store/auth";

export type SignInRequest = { email: string; password: string };
export type SignUpRequest = { email: string; password: string };

export type AuthResponse = {
    accessToken: string;
    user: User;
};

export type AccessTokenResponse = {
    accessToken: string;
};
