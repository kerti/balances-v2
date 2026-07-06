// Pure helpers + the restore data layer for the backup feature (ADR-0036). The
// pure helpers are kept out of the component so they're unit-testable under the
// node-environment vitest runner (ADR-0021); UI behaviour is covered by the
// Playwright @smoke spec.
import { ApiError, isEnvelope, type ErrorEnvelope } from "@/api/client";

// RestoreSummary mirrors the backend backup.Summary: what a backup contains.
export type RestoreSummary = {
  household_name: string;
  // format_version is the version after migration (what this build now holds the
  // data at); source_format_version is the file's on-disk version. source <
  // format means the file was made by an older Balances and upgraded on the way
  // in (#258) — the preview surfaces a reassurance note.
  format_version: number;
  source_format_version: number;
  fidelity: "full" | "compacted";
  counts: Record<string, number>;
};

// RestorePreview is the non-destructive preview: what the uploaded backup will
// load (`backup`) and what the caller's current household will lose (`current`).
// The UI scales its confirmation to the stakes in `current`.
export type RestorePreview = {
  backup: RestoreSummary;
  current: Record<string, number>;
};

// RestoreResult is the commit response — `restored` is always true on success.
export type RestoreResult = {
  restored: boolean;
  summary: RestoreSummary;
};

export type Fidelity = "full" | "compacted";

// triggerDownload saves a blob to disk via a transient object URL + anchor —
// the standard browser-download dance, since the export endpoint streams a
// file rather than JSON the api() client could parse.
function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// downloadBackup fetches and saves a household backup. Shared by BackupCard
// (the export feature itself) and EraseCard (the "download a backup first"
// nudge on the erasure confirm step, ADR-0040) — both just want the file on
// disk, not a preview of its contents.
export async function downloadBackup(fidelity: Fidelity = "full"): Promise<void> {
  const res = await fetch(`/api/backup/export?fidelity=${fidelity}`);
  if (!res.ok) throw new Error(`export failed (${res.status})`);
  const blob = await res.blob();
  triggerDownload(blob, filenameFromDisposition(res.headers.get("Content-Disposition")));
}

// totalRows sums a section-count map — the single headline number the UI shows
// ("12 items") instead of enumerating every section.
export function totalRows(counts: Record<string, number>): number {
  return Object.values(counts).reduce((sum, n) => sum + n, 0);
}

// isHouseholdEmpty reports whether a section-count map represents an empty
// household. Drives the stakes-scaled confirmation: an empty current household
// (a fresh self-host import) is confirmed with a checkbox; a populated one
// requires typing the erase word.
export function isHouseholdEmpty(counts: Record<string, number>): boolean {
  return totalRows(counts) === 0;
}

// postRestore uploads a backup to a restore step (preview or commit) as
// multipart, bypassing the JSON `api` wrapper (which would clobber the multipart
// boundary). Any non-2xx is thrown as an ApiError carrying the ADR-0027
// envelope, so errorMessage() can translate the backup-specific codes. Unlike
// the snapshot importer, a 422 here is a real error (too-new format / invalid
// graph), not a success-shaped result.
async function postRestore<T>(step: "preview" | "commit", file: File): Promise<T> {
  const body = new FormData();
  body.append("file", file);
  const res = await fetch(`/api/backup/restore/${step}`, {
    method: "POST",
    body,
  });
  if (!res.ok) {
    // Read the body once. The previous form called res.json() first, which
    // consumes the stream, so the res.text() fallback always read an empty,
    // already-drained body — the raw-text path was dead (#185). Pull the text
    // ourselves and parse it, so a non-JSON error body (a gateway's HTML page,
    // plain text) is surfaced instead of silently lost.
    const raw = await res.text().catch(() => undefined);
    let errBody: ErrorEnvelope | string | undefined;
    if (raw) {
      try {
        const parsed: unknown = JSON.parse(raw);
        // Valid JSON: keep it only if it's an ADR-0027 envelope; a non-envelope
        // shape is dropped so errorMessage() falls through to the generic copy.
        errBody = isEnvelope(parsed) ? parsed : undefined;
      } catch {
        // Not JSON — surface the raw text.
        errBody = raw;
      }
    }
    throw new ApiError(res.status, res.statusText || `restore failed (${res.status})`, errBody);
  }
  return (await res.json()) as T;
}

export function postRestorePreview(file: File): Promise<RestorePreview> {
  return postRestore<RestorePreview>("preview", file);
}

export function postRestoreCommit(file: File): Promise<RestoreResult> {
  return postRestore<RestoreResult>("commit", file);
}

// filenameFromDisposition pulls the server-suggested name out of a
// Content-Disposition header (e.g. `attachment; filename="household-backup-….json.gz"`),
// falling back to a date-stamped default when the header is missing or unusual.
// Handles both the plain `filename=` and the RFC 5987 `filename*=UTF-8''` forms.
export function filenameFromDisposition(header: string | null, now: Date = new Date()): string {
  const fallback = `household-backup-${now.toISOString().slice(0, 10)}.json.gz`;
  if (!header) return fallback;
  const m = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(header);
  return m?.[1] ?? fallback;
}
