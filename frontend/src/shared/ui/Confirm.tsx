import { Modal } from "./Modal";

type ConfirmProps = {
    open: boolean;
    title?: string;
    message?: string;
    confirmText?: string;
    cancelText?: string;
    loading?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
};

export function Confirm({
                            open,
                            title = "Confirm",
                            message = "Are you sure?",
                            confirmText = "Delete",
                            cancelText = "Cancel",
                            loading = false,
                            onConfirm,
                            onCancel,
                        }: ConfirmProps) {
    return (
        <Modal open={open} onClose={onCancel} title={title} size="sm"
               footer={
                   <>
                       <button className="btn btn-ghost" onClick={onCancel} disabled={loading}>
                           {cancelText}
                       </button>
                       <button className="btn btn-primary" onClick={onConfirm} disabled={loading}>
                           {loading ? "Working..." : confirmText}
                       </button>
                   </>
               }
        >
            <p className="text-sm text-zinc-600 dark:text-zinc-400">{message}</p>
        </Modal>
    );
}
