// The snapshot-import dialog checks a file the moment it is picked or dropped
// (#829), reports new / replaced / unchanged months, and offers one Import
// action that stays off when there is nothing to write.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useMutation } from "@tanstack/react-query";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { postSnapshotImport, type ImportArgs, type ImportResult } from "@/hooks/snapshotImport";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

const XLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
const BASE = "/api/assets/a1/snapshots";

function Harness() {
  const mutation = useMutation({
    mutationFn: (args: ImportArgs) => postSnapshotImport(BASE, args.file, args.mode),
  });
  return (
    <ImportSnapshotsDialog
      templateUrl={`${BASE}/import-template`}
      mutation={mutation}
      currency="IDR"
    />
  );
}

function preview(counts: Partial<ImportResult>): ImportResult {
  return {
    mode: "preview",
    committed: false,
    to_insert: 0,
    to_update: 0,
    unchanged: 0,
    errors: [],
    ...counts,
  };
}

function xlsx(name: string) {
  return new File([new Uint8Array([1])], name, { type: XLSX });
}

// Serves `previews` in order (the last one repeats) and records each request's mode.
function stub(previews: ImportResult[]) {
  const modes: string[] = [];
  server.use(
    http.post(`${BASE}/import`, ({ request }) => {
      const mode = new URL(request.url).searchParams.get("mode") ?? "";
      modes.push(mode);
      if (mode === "commit") {
        return HttpResponse.json({
          ...previews[0],
          mode: "commit",
          committed: true,
        });
      }
      return HttpResponse.json(previews[Math.min(modes.length - 1, previews.length - 1)]);
    }),
  );
  return modes;
}

async function openDialog() {
  const user = userEvent.setup();
  renderWithProviders(<Harness />);
  await user.click(screen.getByTestId("import-snapshots-trigger"));
  return user;
}

beforeEach(() => {
  vi.clearAllMocks();
});

// covers: INV-IMPORT-11
describe("ImportSnapshotsDialog check on pick", () => {
  it("has no Check file button; picking a file runs the preview and enables Import", async () => {
    const modes = stub([preview({ to_insert: 3 })]);
    const user = await openDialog();

    expect(screen.queryByTestId("import-check-btn")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /check file/i })).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
    expect(screen.getByTestId("import-commit-btn")).toHaveTextContent(/^Import$/);

    await user.upload(screen.getByTestId("import-file-input"), xlsx("months.xlsx"));

    await screen.findByTestId("import-preview");
    expect(modes).toEqual(["preview"]);
    expect(screen.getByTestId("import-commit-btn")).toBeEnabled();
    // Still just "Import", whatever the counts.
    expect(screen.getByTestId("import-commit-btn")).toHaveTextContent(/^Import$/);
  });

  it("runs the preview on a dropped file too", async () => {
    const modes = stub([preview({ to_insert: 1 })]);
    await openDialog();

    fireEvent.drop(screen.getByTestId("import-drop-zone"), {
      dataTransfer: { files: [xlsx("dropped.xlsx")] },
    });

    await screen.findByTestId("import-preview");
    expect(modes).toEqual(["preview"]);
    expect(screen.getByTestId("import-commit-btn")).toBeEnabled();
  });

  it("shows the filename once and keeps the native input out of sight but reachable", async () => {
    stub([preview({ to_insert: 1 })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("months.xlsx"));
    await screen.findByTestId("import-preview");

    expect(screen.getAllByText("months.xlsx")).toHaveLength(1);
    const input = screen.getByTestId("import-file-input");
    expect(input).toHaveClass("sr-only");
    expect(input).not.toHaveAttribute("hidden");
    expect(screen.getByText("Choose a file")).toHaveAttribute("for", input.id);
  });

  it("shows row errors on pick and keeps Import disabled", async () => {
    stub([preview({ errors: [{ row: 4, message: "bad amount" }] })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("bad.xlsx"));

    const errors = await screen.findByTestId("import-errors");
    expect(errors).toHaveTextContent("Row 4: bad amount");
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
  });

  it("re-runs the preview when another file is picked", async () => {
    const modes = stub([
      preview({ errors: [{ row: 2, message: "bad amount" }] }),
      preview({ to_insert: 2 }),
    ]);
    const user = await openDialog();

    await user.upload(screen.getByTestId("import-file-input"), xlsx("first.xlsx"));
    await screen.findByTestId("import-errors");

    await user.upload(screen.getByTestId("import-file-input"), xlsx("second.xlsx"));
    await screen.findByTestId("import-preview");
    expect(modes).toEqual(["preview", "preview"]);
    expect(screen.queryByTestId("import-errors")).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toBeEnabled();
    expect(screen.getByTestId("import-selected-file")).toHaveTextContent("second.xlsx");
  });

  it("refuses a non-xlsx without calling the server", async () => {
    const modes = stub([preview({ to_insert: 1 })]);
    await openDialog();
    fireEvent.drop(screen.getByTestId("import-drop-zone"), {
      dataTransfer: { files: [new File(["x"], "notes.txt", { type: "text/plain" })] },
    });

    expect(await screen.findByTestId("import-invalid-file")).toBeInTheDocument();
    expect(modes).toEqual([]);
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
  });

  it("commits with the picked file when Import is pressed", async () => {
    const modes = stub([preview({ to_insert: 1, to_update: 1 })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("months.xlsx"));
    await screen.findByTestId("import-preview");

    await user.click(screen.getByTestId("import-commit-btn"));

    await screen.findByTestId("import-done");
    expect(modes).toEqual(["preview", "commit"]);
  });
});

// covers: INV-IMPORT-12
describe("ImportSnapshotsDialog new / replaced / unchanged", () => {
  it("reports the three counts, pluralised", async () => {
    stub([preview({ to_insert: 3, to_update: 2, unchanged: 22 })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("months.xlsx"));

    expect(await screen.findByTestId("import-preview")).toHaveTextContent(
      "Ready: 3 new months, 2 replaced, 22 unchanged.",
    );
    expect(screen.getByTestId("import-commit-btn")).toBeEnabled();
  });

  it("uses the singular for exactly one new month", async () => {
    stub([preview({ to_insert: 1, to_update: 0, unchanged: 1 })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("months.xlsx"));

    expect(await screen.findByTestId("import-preview")).toHaveTextContent(
      "Ready: 1 new month, 1 unchanged.",
    );
  });

  it("leaves out the counts that are zero", async () => {
    stub([preview({ to_insert: 0, to_update: 1, unchanged: 26 })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("months.xlsx"));

    const text = (await screen.findByTestId("import-preview")).textContent;
    expect(text).toContain("Ready: 1 replaced, 26 unchanged.");
    expect(text).not.toContain("new month");
  });

  it("stays enabled when only replacements are pending", async () => {
    stub([preview({ to_update: 1, unchanged: 5 })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("months.xlsx"));

    await screen.findByTestId("import-preview");
    expect(screen.getByTestId("import-commit-btn")).toBeEnabled();
  });

  it("says there is nothing to import when every month already matches", async () => {
    const modes = stub([preview({ unchanged: 27 })]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("own-export.xlsx"));

    expect(await screen.findByTestId("import-all-unchanged")).toHaveTextContent(
      "Nothing to import — every month already matches",
    );
    expect(screen.queryByTestId("import-preview")).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
    expect(modes).toEqual(["preview"]);
  });

  it("keeps the 'no rows' message for a file with nothing in it", async () => {
    stub([preview({})]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("empty.xlsx"));

    expect(await screen.findByTestId("import-result")).toHaveTextContent("No rows found");
    expect(screen.queryByTestId("import-all-unchanged")).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
  });
});
