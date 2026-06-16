// Pure helpers for the backup feature (ADR-0036). Kept out of the component so
// they're unit-testable under the node-environment vitest runner (ADR-0021);
// UI behaviour is covered by the Playwright @smoke spec.

// filenameFromDisposition pulls the server-suggested name out of a
// Content-Disposition header (e.g. `attachment; filename="household-backup-….json.gz"`),
// falling back to a date-stamped default when the header is missing or unusual.
// Handles both the plain `filename=` and the RFC 5987 `filename*=UTF-8''` forms.
export function filenameFromDisposition(
  header: string | null,
  now: Date = new Date(),
): string {
  const fallback = `household-backup-${now.toISOString().slice(0, 10)}.json.gz`
  if (!header) return fallback
  const m = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(header)
  return m?.[1] ?? fallback
}
