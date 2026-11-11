// Picking or dropping a workbook checks it at once (#829): there is no Check
// file button, the filename shows once, and Create is the only action.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useMutation } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { ImportPositionDialog } from "@/components/dialogs/ImportPositionDialog";
import { postCreateImport, type CreateImportArgs } from "@/hooks/snapshotImport";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

const XLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

function Harness() {
  const mutation = useMutation({
    mutationFn: (args: CreateImportArgs) =>
      postCreateImport("/api/bank-accounts", args.file, args.mode),
  });
  return <ImportPositionDialog noun="account" mutation={mutation} />;
}

const clean = {
  mode: "preview",
  committed: false,
  would_create: true,
  to_insert: 4,
  field_errors: [],
  errors: [],
  name: "Everyday",
  matches: [],
};

const bad = {
  ...clean,
  would_create: false,
  field_errors: [{ field: "display_name", message: "is required" }],
};

function xlsx(name: string) {
  return new File([new Uint8Array([1])], name, { type: XLSX });
}

// Serves `previews` in order and records each request's mode.
function stub(previews: object[]) {
  const modes: string[] = [];
  server.use(
    http.post("/api/bank-accounts/import", ({ request }) => {
      const mode = new URL(request.url).searchParams.get("mode") ?? "";
      modes.push(mode);
      if (mode === "commit") {
        return HttpResponse.json({ ...clean, mode: "commit", committed: true, position_id: "p1" });
      }
      return HttpResponse.json(previews[Math.min(modes.length - 1, previews.length - 1)]);
    }),
  );
  return modes;
}

async function openDialog() {
  const user = userEvent.setup();
  renderWithProviders(
    <MemoryRouter>
      <Harness />
    </MemoryRouter>,
  );
  await user.click(screen.getByTestId("import-position-trigger"));
  return user;
}

beforeEach(() => {
  vi.clearAllMocks();
});

// covers: INV-IMPORT-11
describe("ImportPositionDialog check on pick", () => {
  it("has no Check file button and checks the file the moment it is picked", async () => {
    const modes = stub([clean]);
    const user = await openDialog();

    expect(screen.queryByTestId("import-check-btn")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /check file/i })).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();

    await user.upload(screen.getByTestId("import-file-input"), xlsx("everyday.xlsx"));

    await screen.findByTestId("import-preview");
    expect(modes).toEqual(["preview"]);
    expect(screen.getByTestId("import-commit-btn")).toBeEnabled();
    expect(screen.getByTestId("import-commit-btn")).toHaveTextContent(/^Create$/);
  });

  it("checks a dropped file the same way", async () => {
    const modes = stub([clean]);
    await openDialog();

    fireEvent.drop(screen.getByTestId("import-drop-zone"), {
      dataTransfer: { files: [xlsx("dropped.xlsx")] },
    });

    await screen.findByTestId("import-preview");
    expect(modes).toEqual(["preview"]);
    expect(screen.getByTestId("import-selected-file")).toHaveTextContent("dropped.xlsx");
  });

  it("shows the filename once and keeps the native input out of sight but reachable", async () => {
    stub([clean]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("everyday.xlsx"));
    await screen.findByTestId("import-preview");

    expect(screen.getAllByText("everyday.xlsx")).toHaveLength(1);
    const input = screen.getByTestId("import-file-input");
    expect(input).toHaveClass("sr-only");
    expect(input).not.toHaveAttribute("hidden");
    expect(input).not.toHaveAttribute("tabindex", "-1");
    // The visible control is a label for the input, so a click opens the picker.
    expect(screen.getByText("Choose a file")).toHaveAttribute("for", input.id);
  });

  it("shows the errors on pick and keeps Create disabled", async () => {
    const modes = stub([bad]);
    const user = await openDialog();
    await user.upload(screen.getByTestId("import-file-input"), xlsx("bad.xlsx"));

    await screen.findByTestId("import-errors");
    expect(modes).toEqual(["preview"]);
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
  });

  it("re-checks when another file is picked", async () => {
    const modes = stub([bad, clean]);
    const user = await openDialog();

    await user.upload(screen.getByTestId("import-file-input"), xlsx("first.xlsx"));
    await screen.findByTestId("import-errors");
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();

    await user.upload(screen.getByTestId("import-file-input"), xlsx("second.xlsx"));
    await screen.findByTestId("import-preview");
    expect(modes).toEqual(["preview", "preview"]);
    expect(screen.queryByTestId("import-errors")).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toBeEnabled();
    expect(screen.getByTestId("import-selected-file")).toHaveTextContent("second.xlsx");
  });

  it("refuses a non-xlsx without calling the server", async () => {
    const modes = stub([clean]);
    await openDialog();
    fireEvent.drop(screen.getByTestId("import-drop-zone"), {
      dataTransfer: { files: [new File(["x"], "notes.txt", { type: "text/plain" })] },
    });

    expect(await screen.findByTestId("import-invalid-file")).toBeInTheDocument();
    expect(modes).toEqual([]);
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
  });
});
