import { ApiError, isEnvelope, type ErrorEnvelope } from "@/api/client";

// Shared core for the bulk snapshot importer, group-agnostic. Every amount-shape
// position group (assets, liabilities, receivables) exposes the same backend
// endpoints under its own `{base}/import-template` + `{base}/import` — only the
// base path and the cache-invalidation differ, so those stay in the per-group
// hook files while the wire types + transport live here.

export type ImportRowError = { row: number; message: string };

export type ImportResult = {
  mode: "preview" | "commit";
  committed: boolean;
  to_insert: number;
  to_update: number;
  errors: ImportRowError[];
};

export type ImportArgs = { file: File; mode: "preview" | "commit" };

// ----- create-from-file import (position workbook -> new position) ----------

export type ImportFieldError = { field: string; message: string };

// CreateImportResult is the create-import counterpart of ImportResult: it adds
// the Detail-sheet half (field_errors + would_create) and, on a committed
// write, the new position's id. ToInsert counts the seeded snapshots; a new
// position has no existing months, so there is no to_update. ledger_to_insert
// counts the seeded transaction ledger (investment subtypes only, issue #90);
// absent for the snapshot-only groups.
export type CreateImportResult = {
  mode: "preview" | "commit";
  committed: boolean;
  would_create: boolean;
  position_id?: string;
  to_insert: number;
  ledger_to_insert?: number;
  field_errors: ImportFieldError[];
  errors: ImportRowError[];
};

export type CreateImportArgs = { file: File; mode: "preview" | "commit" };

// postCreateImport uploads a position workbook to a group's create-import
// endpoint (e.g. `/api/bank-accounts`). Like postSnapshotImport it bypasses the
// JSON `api` wrapper for multipart, and treats a 422 as the "workbook had bad
// fields/rows" outcome whose body is a valid CreateImportResult.
export async function postCreateImport(
  base: string,
  file: File,
  mode: "preview" | "commit",
): Promise<CreateImportResult> {
  const body = new FormData();
  body.append("file", file);
  const res = await fetch(`${base}/import?mode=${mode}`, {
    method: "POST",
    body,
  });
  if (res.status === 422) return (await res.json()) as CreateImportResult;
  if (!res.ok) {
    let errBody: ErrorEnvelope | string | undefined;
    try {
      const parsed = await res.json();
      errBody = isEnvelope(parsed) ? parsed : undefined;
    } catch {
      errBody = await res.text().catch(() => undefined);
    }
    throw new ApiError(res.status, res.statusText || `import failed (${res.status})`, errBody);
  }
  return (await res.json()) as CreateImportResult;
}

// snapshotImportTemplateUrl is a plain GET the browser can hit as a download
// link; the session cookie rides along same-origin. `base` is the snapshots
// collection path, e.g. `/api/assets/{id}/snapshots`.
export function snapshotImportTemplateUrl(base: string): string {
  return `${base}/import-template`;
}

// postSnapshotImport uploads multipart, so it bypasses the JSON `api` wrapper
// (which would force a Content-Type and clobber the multipart boundary). A 422
// is the "file had bad rows" outcome — its body is a valid ImportResult, so we
// return it rather than throwing, letting the dialog render the per-row errors.
export async function postSnapshotImport(
  base: string,
  file: File,
  mode: "preview" | "commit",
): Promise<ImportResult> {
  const body = new FormData();
  body.append("file", file);
  const res = await fetch(`${base}/import?mode=${mode}`, {
    method: "POST",
    body,
  });
  if (res.status === 422) return (await res.json()) as ImportResult;
  if (!res.ok) {
    let errBody: ErrorEnvelope | string | undefined;
    try {
      const parsed = await res.json();
      errBody = isEnvelope(parsed) ? parsed : undefined;
    } catch {
      errBody = await res.text().catch(() => undefined);
    }
    throw new ApiError(res.status, res.statusText || `import failed (${res.status})`, errBody);
  }
  return (await res.json()) as ImportResult;
}
