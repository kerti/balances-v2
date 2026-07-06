import { describe, it, expect } from "vitest";

import { fileFromDrop, isXlsxFile } from "@/lib/importDrop";

// Node's vitest env has no DOM File, but the global File constructor is
// available; these build minimal stand-ins for the three drop paths
// (dragover→leave→drop reduce to: what file, if any, did the drop carry).
function makeFile(name: string, type = ""): File {
  return new File([new Uint8Array([1])], name, { type });
}

// covers: INV-IMPORT-06
describe("isXlsxFile", () => {
  it("accepts a .xlsx by extension regardless of (often empty) MIME", () => {
    expect(isXlsxFile(makeFile("balances.xlsx"))).toBe(true);
    expect(isXlsxFile(makeFile("Balances.XLSX"))).toBe(true);
  });

  it("accepts the spreadsheet MIME even if the name lacks the extension", () => {
    const mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
    expect(isXlsxFile(makeFile("download", mime))).toBe(true);
  });

  it("rejects non-spreadsheet files", () => {
    expect(isXlsxFile(makeFile("statement.pdf", "application/pdf"))).toBe(false);
    expect(isXlsxFile(makeFile("data.csv", "text/csv"))).toBe(false);
  });
});

describe("fileFromDrop", () => {
  it("returns the first file when a single .xlsx is dropped", () => {
    const file = makeFile("q1.xlsx");
    expect(fileFromDrop([file])).toEqual({ ok: true, file });
  });

  it("takes only the first file from a multi-file drop", () => {
    const first = makeFile("a.xlsx");
    const out = fileFromDrop([first, makeFile("b.xlsx")]);
    expect(out).toEqual({ ok: true, file: first });
  });

  it("flags a non-.xlsx drop as invalid", () => {
    expect(fileFromDrop([makeFile("notes.pdf", "application/pdf")])).toEqual({
      ok: false,
      reason: "invalid",
    });
  });

  it("reports an empty drop (no files) without clobbering selection", () => {
    expect(fileFromDrop([])).toEqual({ ok: false, reason: "empty" });
    expect(fileFromDrop(null)).toEqual({ ok: false, reason: "empty" });
  });
});
