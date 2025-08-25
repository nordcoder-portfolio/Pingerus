import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import type { Check } from "../../entities/check/types";

const schema = z.object({
    url: z.string().url("Введите корректный URL").min(4).max(2048),
    intervalSec: z.number().int().min(10, "Минимум 10 сек").max(86400, "Не более 86400"),
});
type FormValues = z.infer<typeof schema>;

type Props = {
    initial?: Check | null;
    onSubmit: (values: FormValues) => Promise<void> | void;
    submitting?: boolean;
};

export function CheckForm({ initial, onSubmit, submitting }: Props) {
    const {
        register,
        handleSubmit,
        formState: { errors },
    } = useForm<FormValues>({
        resolver: zodResolver(schema),
        defaultValues: initial
            ? { url: initial.url, intervalSec: initial.intervalSec }
            : { url: "", intervalSec: 60 },
    });

    return (
        <form className="space-y-4" onSubmit={handleSubmit(onSubmit)}>
            <div>
                <label className="label">URL</label>
                <input className="input" placeholder="https://example.com/health" {...register("url")} />
                {errors.url && <p className="mt-1 text-xs text-rose-600">{errors.url.message}</p>}
            </div>

            <div>
                <label className="label">Interval (sec)</label>
                <input
                    className="input"
                    type="number"
                    min={10}
                    max={86400}
                    {...register("intervalSec", { valueAsNumber: true })}
                />
                {errors.intervalSec && (
                    <p className="mt-1 text-xs text-rose-600">{errors.intervalSec.message}</p>
                )}
            </div>

            <button className="btn btn-primary w-full" type="submit" disabled={!!submitting}>
                {submitting ? "Saving..." : initial ? "Save changes" : "Create check"}
            </button>
        </form>
    );
}
