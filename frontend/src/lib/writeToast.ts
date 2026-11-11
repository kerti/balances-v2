// One toast per user-started write (ADR-0058 §3-§5). Every hook and call site
// goes through these helpers so durations, the Undo action and the error copy
// stay in one place. A plain toast lasts 4 s; a toast with an action lasts 8 s
// (sonner pauses the timer while the pointer is over it).
import type React from "react";
import { toast } from "sonner";
import type { QueryClient } from "@tanstack/react-query";
import i18n from "@/i18n";
import { api, ApiError } from "@/api/client";
import { errorMessage } from "@/lib/errorMessage";
import { formatNumber } from "@/lib/format";

export const PLAIN_TOAST_MS = 4000;
export const UNDO_TOAST_MS = 8000;

export function toastSaved(kind: "saved" | "added" = "saved") {
  toast.success(i18n.t(`common:toast.${kind}`), { duration: PLAIN_TOAST_MS });
}

// "Saved 7 · 2 replaced earlier values": a bulk save has no Undo (ADR-0058 §2).
// With `next` the toast carries the next step (ADR-0058 §6, #805): an action
// "Next: {group} ({n}) →" and the longer, hover-pausing action duration.
export type BulkNext = { group: string; count: number; onClick: () => void };

export function toastBulkSaved(written: number, replaced: number, next?: BulkNext) {
  const message =
    replaced > 0
      ? i18n.t("common:toast.bulkSavedReplaced", { written, count: replaced })
      : i18n.t("common:toast.bulkSaved", { count: written });
  if (!next) {
    toast.success(message, { duration: PLAIN_TOAST_MS });
    return;
  }
  toast.success(message, {
    testId: "toast-bulk-saved",
    duration: UNDO_TOAST_MS,
    action: {
      label: i18n.t("common:toast.bulkNext", {
        group: next.group,
        n: formatNumber(next.count),
      }),
      onClick: next.onClick,
    },
  });
}

export function toastWriteError(err: unknown) {
  toast.error(errorMessage(err));
}

// A snapshot edit: when the PATCH archived a displaced row (ADR-0058 §1) the
// toast carries Undo naming both rows; an identical edit just says Saved.
export function toastSnapshotEdit(
  qc: QueryClient,
  snapshotsPath: string,
  saved: { id: string; replaced_id: string | null },
  onUndone: () => void,
) {
  if (!saved.replaced_id) {
    toastSaved();
    return;
  }
  toastUndoable({
    message: i18n.t("common:toast.replaced"),
    undoPath: `${snapshotsPath}/${saved.replaced_id}/undo-replace`,
    undoBody: { replacement_id: saved.id },
    testId: "toast-replaced",
    qc,
    onUndone,
  });
}

// A snapshot move (#791): always one Undo toast. The undo route hangs off the
// original (the archived row to bring back) and names the moved row plus the
// target month's displaced row, if the move replaced one.
export function toastSnapshotMove(
  qc: QueryClient,
  snapshotsPath: string,
  originalId: string,
  moved: { id: string; replaced_id: string | null },
  month: string,
  onUndone: () => void,
) {
  toastUndoable({
    message: i18n.t(moved.replaced_id ? "common:toast.movedReplaced" : "common:toast.moved", {
      month,
    }),
    undoPath: `${snapshotsPath}/${originalId}/undo-move`,
    undoBody: { moved_id: moved.id, displaced_id: moved.replaced_id },
    testId: "toast-moved",
    qc,
    onUndone,
  });
}

type UndoableOptions = {
  message: string;
  // POST that reverses the write; the body is JSON when given.
  undoPath: string;
  undoBody?: unknown;
  // POST unless the reversing write is a PUT (a tag re-assignment, #853).
  undoMethod?: "POST" | "PUT";
  // data-testid on the toast: "toast-deleted" or "toast-replaced", so a spec
  // can tell a delete's Undo from an edit's when both are on screen.
  testId: string;
  qc: QueryClient;
  // Re-invalidate the queries the original write invalidated. Reports are
  // refreshed here too: an undo goes through api(), not a mutation, so the
  // global MutationCache invalidation never sees it.
  onUndone: () => void;
};

// A delete or single-item replace: says what happened and offers Undo. A failed
// undo (e.g. the slot was re-filled, 409) becomes a red toast with the server's
// localised message; a successful one is its own short confirmation.
export function toastUndoable({
  message,
  undoPath,
  undoBody,
  undoMethod = "POST",
  testId,
  qc,
  onUndone,
}: UndoableOptions) {
  toast(message, {
    testId,
    duration: UNDO_TOAST_MS,
    // The countdown bar (index.css) runs for exactly the toast's duration.
    className: "undo-countdown",
    style: { "--undo-countdown-ms": `${UNDO_TOAST_MS}ms` } as React.CSSProperties,
    action: {
      label: i18n.t("common:toast.undo"),
      onClick: () => {
        api(undoPath, {
          method: undoMethod,
          ...(undoBody === undefined ? {} : { body: JSON.stringify(undoBody) }),
        })
          .then(() => {
            onUndone();
            qc.invalidateQueries({ queryKey: ["reports"] });
            toast.success(i18n.t("common:toast.undone"), { duration: PLAIN_TOAST_MS });
          })
          .catch(toastWriteError);
      },
    },
  });
}

// Long or background work: one loading toast that resolves in place to success
// or error. `success` returning null dismisses the toast instead, for a result
// the screen already shows. `inlineError` is for a caller whose open form shows
// the error itself (ADR-0058 §5): the toast is dismissed, not turned red.
export async function withLoadingToast<T>(
  loading: string,
  work: () => Promise<T>,
  success: (result: T) => string | null,
  opts: { fallbackError?: string; inlineError?: boolean } = {},
): Promise<T> {
  const id = toast.loading(loading);
  try {
    const result = await work();
    const message = success(result);
    if (message === null) toast.dismiss(id);
    else toast.success(message, { id, duration: PLAIN_TOAST_MS });
    return result;
  } catch (err) {
    if (opts.inlineError) toast.dismiss(id);
    else {
      // A native Error (a failed fetch, a non-2xx without an envelope) carries
      // English noise; only a server code earns its own message.
      const message =
        err instanceof ApiError ? errorMessage(err) : errorMessage(null, opts.fallbackError);
      toast.error(message, { id });
    }
    throw err;
  }
}
