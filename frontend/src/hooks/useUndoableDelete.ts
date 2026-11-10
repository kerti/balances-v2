import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { toastUndoable, toastWriteError } from "@/lib/writeToast";

type Config<V> = {
  // The DELETE route; the undo route is always `${path}/undo-delete` (#821).
  path: (variables: V) => string;
  message: (variables: V) => string;
  invalidate: (qc: QueryClient, variables: V) => void;
};

// A single-item delete that ends in one Undo toast (ADR-0058 §3-§4). A failed
// delete has no form open, so the error is a red toast too.
export function useUndoableDelete<V = string>(cfg: Config<V>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (variables: V) => api(cfg.path(variables), { method: "DELETE" }),
    onSuccess: (_data, variables) => {
      cfg.invalidate(qc, variables);
      toastUndoable({
        message: cfg.message(variables),
        undoPath: `${cfg.path(variables)}/undo-delete`,
        testId: "toast-deleted",
        qc,
        onUndone: () => cfg.invalidate(qc, variables),
      });
    },
    onError: toastWriteError,
  });
}
export type PositionDelete = { id: string; name: string };
