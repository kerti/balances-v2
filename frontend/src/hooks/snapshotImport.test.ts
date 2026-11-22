import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/api/client";
import {
  postCreateImport,
  postSnapshotImport,
  type CreateImportResult,
} from "@/hooks/snapshotImport";

// postCreateImport is the transport behind the create-from-file dialog. These
// cover its three branches — ok, the 422 "bad workbook" result, and a hard
// failure — plus the URL/mode it builds. The dialog component itself is pure
// glue over this + react-query; the meaningful logic lives here (and the repo
// has no jsdom/RTL runner yet, ADR-0021).

// A committed import raises a loading toast; sonner needs a DOM, so stub it.
vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const cleanPreview: CreateImportResult = {
  mode: "preview",
  committed: false,
  would_create: true,
  to_insert: 2,
  field_errors: [],
  errors: [],
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe("postCreateImport", () => {
  it("posts multipart to {base}/import with the mode query and returns the result", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, cleanPreview));
    vi.stubGlobal("fetch", fetchMock);

    const file = new File(["x"], "acct.xlsx");
    const result = await postCreateImport("/api/bank-accounts", file, "preview");

    expect(result).toEqual(cleanPreview);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/bank-accounts/import?mode=preview");
    expect(init.method).toBe("POST");
    expect(init.body).toBeInstanceOf(FormData);
    expect((init.body as FormData).get("file")).toBe(file);
  });

  it("treats a 422 as a CreateImportResult (bad workbook), not a throw", async () => {
    const body: CreateImportResult = {
      mode: "commit",
      committed: false,
      would_create: false,
      to_insert: 0,
      field_errors: [
        {
          field: "sole_owner",
          message: "no household member has the email x@y.z",
        },
      ],
      errors: [{ row: 2, message: "amount is required" }],
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(422, body)));

    const result = await postCreateImport(
      "/api/bank-accounts",
      new File(["x"], "a.xlsx"),
      "commit",
    );
    expect(result.would_create).toBe(false);
    expect(result.field_errors).toHaveLength(1);
    expect(result.errors).toHaveLength(1);
  });

  it("throws ApiError on a non-ok, non-422 response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(500, { code: "INTERNAL" })));
    await expect(
      postCreateImport("/api/bank-accounts", new File(["x"], "a.xlsx"), "commit"),
    ).rejects.toBeInstanceOf(ApiError);
  });

  // covers: INV-CONTRACT-06
  it("throws a 422 ADR-0027 envelope (VALUE_OUT_OF_RANGE) instead of reading it as a result", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(422, { code: "VALUE_OUT_OF_RANGE" })),
    );
    const err = await postCreateImport(
      "/api/bank-accounts",
      new File(["x"], "a.xlsx"),
      "preview",
    ).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).body).toEqual({ code: "VALUE_OUT_OF_RANGE" });
  });
});

describe("postSnapshotImport", () => {
  it("treats a 422 body without a code as the file's result", async () => {
    const body = {
      mode: "preview",
      committed: false,
      to_insert: 0,
      errors: [{ row: 2, message: "x" }],
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(422, body)));
    const r = await postSnapshotImport(
      "/api/assets/1/snapshots",
      new File(["x"], "a.xlsx"),
      "preview",
    );
    expect(r.errors).toHaveLength(1);
  });

  // covers: INV-CONTRACT-06
  it("throws a 422 ADR-0027 envelope (VALUE_OUT_OF_RANGE) instead of reading it as a result", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(422, { code: "VALUE_OUT_OF_RANGE" })),
    );
    await expect(
      postSnapshotImport("/api/assets/1/snapshots", new File(["x"], "a.xlsx"), "preview"),
    ).rejects.toBeInstanceOf(ApiError);
  });
});
