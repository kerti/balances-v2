// Pure drop-target logic for the bulk snapshot importer, kept out of the
// component so it's testable under the node-only vitest runner (ADR-0021).
// Both the OS file-picker and the drag-and-drop zone funnel their candidate
// file through here, so a non-.xlsx is rejected identically either way.

const XLSX_EXT = ".xlsx";
const XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

// A dropped file's `type` is unreliable across OSes (often empty), so the
// extension is the primary signal; the MIME check is a belt-and-braces second.
export function isXlsxFile(file: File): boolean {
  return file.name.toLowerCase().endsWith(XLSX_EXT) || file.type === XLSX_MIME;
}

export type DropOutcome =
  // `empty` means the drop carried no files (e.g. dragging selected text); the
  // caller leaves the current selection untouched rather than flagging it bad.
  { ok: false; reason: "empty" } | { ok: false; reason: "invalid" } | { ok: true; file: File };

// fileFromDrop normalises a DataTransfer.files (or the picker's FileList) into
// a verdict. Only the first file is taken — the importer is single-file.
export function fileFromDrop(files: FileList | File[] | null): DropOutcome {
  const list = files ? Array.from(files) : [];
  if (list.length === 0) return { ok: false, reason: "empty" };
  const file = list[0];
  if (!isXlsxFile(file)) return { ok: false, reason: "invalid" };
  return { ok: true, file };
}
