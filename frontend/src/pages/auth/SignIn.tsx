import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useNavigate } from "react-router-dom";
import { signIn } from "../../entities/auth/api";

const schema = z.object({
    email: z.string().email("Некорректный email"),
    password: z.string().min(1, "Введите пароль"),
});
type FormValues = z.infer<typeof schema>;

export function SignIn() {
    const navigate = useNavigate();
    const {
        register,
        handleSubmit,
        formState: { errors, isSubmitting },
        setError,
    } = useForm<FormValues>({ resolver: zodResolver(schema) });

    const onSubmit = async (values: FormValues) => {
        try {
            await signIn(values);
            navigate("/checks");
        } catch (e: any) {
            setError("root", {
                message:
                    e?.message?.slice(0, 200) ||
                    "Не удалось войти. Проверьте email/пароль.",
            });
        }
    };

    return (
        <div className="mx-auto max-w-md">
            <div className="card card-hover p-6">
                <h1 className="mb-4 text-2xl font-semibold">Sign In</h1>

                {/** Ошибка формы */}
                {"root" in errors && errors.root?.message && (
                    <div className="mb-3 rounded-xl border border-rose-300 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300">
                        {errors.root.message}
                    </div>
                )}

                <form className="space-y-4" onSubmit={handleSubmit(onSubmit)}>
                    <div>
                        <label className="label">Email</label>
                        <input
                            className="input"
                            type="email"
                            placeholder="you@example.com"
                            {...register("email")}
                        />
                        {errors.email && (
                            <p className="mt-1 text-xs text-rose-600">
                                {errors.email.message}
                            </p>
                        )}
                    </div>

                    <div>
                        <label className="label">Password</label>
                        <input
                            className="input"
                            type="password"
                            placeholder="••••••••"
                            {...register("password")}
                        />
                        {errors.password && (
                            <p className="mt-1 text-xs text-rose-600">
                                {errors.password.message}
                            </p>
                        )}
                    </div>

                    <button
                        className="btn btn-primary w-full"
                        type="submit"
                        disabled={isSubmitting}
                    >
                        {isSubmitting ? "Signing in..." : "Continue"}
                    </button>
                </form>

                <p className="mt-4 text-center text-sm text-zinc-500">
                    No account?{" "}
                    <Link to="/sign-up" className="underline">
                        Sign Up
                    </Link>
                </p>
            </div>
        </div>
    );
}
