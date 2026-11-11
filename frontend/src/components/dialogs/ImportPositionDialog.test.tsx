// The create-from-file import dialog names the position it is about to make and,
// when the household already has one like it, says so (#828, audit BUG-03). The
// notice never blocks: Create anyway commits exactly as the plain Create does.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useMutation } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { ImportPositionDialog } from "@/components/dialogs/ImportPositionDialog";
import { postCreateImport, type CreateImportArgs } from "@/hooks/snapshotImport";
import { routes } from "@/lib/routes";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

const XLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

function Harness() {
  const mutation = useMutation({
    mutationFn: (args: CreateImportArgs) =>
      postCreateImport("/api/bank-accounts", args.file, args.mode),
  });
  return <ImportPositionDialog noun="account" mutation={mutation} />;
}

function renderDialog() {
  return renderWithProviders(
    <MemoryRouter initialEntries={["/assets/bank-accounts"]}>
      <Routes>
        <Route path="/assets/bank-accounts" element={<Harness />} />
        <Route path="*" element={<div data-testid="landed" />} />
      </Routes>
    </MemoryRouter>,
  );
}

const cleanPreview = {
  mode: "preview",
  committed: false,
  would_create: true,
  to_insert: 27,
  field_errors: [],
  errors: [],
  name: "Everyday",
  identifier: "1234567890",
  matches: [],
};

const existing = {
  id: "acct-1",
  name: "Everyday",
  status: "active",
  group: "asset",
  subtype: "bank_account",
};

// Stubs the preview and records every commit POST the dialog makes.
function stubImport(preview: object) {
  const commits: string[] = [];
  server.use(
    http.post("/api/bank-accounts/import", ({ request }) => {
      const mode = new URL(request.url).searchParams.get("mode");
      if (mode === "commit") {
        commits.push(mode);
        return HttpResponse.json({
          ...preview,
          mode: "commit",
          committed: true,
          position_id: "acct-2",
          matches: [],
        });
      }
      return HttpResponse.json(preview);
    }),
  );
  return commits;
}

async function checkFile() {
  const user = userEvent.setup();
  await user.click(screen.getByTestId("import-position-trigger"));
  await user.upload(
    screen.getByTestId("import-file-input"),
    new File([new Uint8Array([1])], "everyday.xlsx", { type: XLSX }),
  );
  await user.click(screen.getByTestId("import-check-btn"));
  await screen.findByTestId("import-preview");
  return user;
}

beforeEach(() => {
  vi.clearAllMocks();
});

// covers: INV-IMPORT-09
describe("ImportPositionDialog duplicate notice", () => {
  it("names the position with a masked account number and the snapshot count", async () => {
    stubImport(cleanPreview);
    renderDialog();
    await checkFile();

    const preview = screen.getByTestId("import-preview");
    expect(preview).toHaveTextContent("Everyday (•••• 7890)");
    expect(preview).toHaveTextContent("27 snapshots");
    // The full number is never put on screen.
    expect(screen.queryByText(/1234567890/)).not.toBeInTheDocument();
  });

  it("names a position that has no identifier by its name alone", async () => {
    stubImport({ ...cleanPreview, name: "Family house", identifier: undefined });
    renderDialog();
    await checkFile();

    const preview = screen.getByTestId("import-preview");
    expect(preview).toHaveTextContent("Family house, with 27 snapshots");
    expect(preview).not.toHaveTextContent("(");
  });

  it("shows no notice and a plain Create when nothing matches", async () => {
    stubImport(cleanPreview);
    renderDialog();
    await checkFile();

    expect(screen.queryByTestId("import-match-notice")).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toHaveTextContent(/^Create$/);
  });

  it("warns on a match, links to its detail page, and Create anyway still commits", async () => {
    const commits = stubImport({ ...cleanPreview, matches: [existing] });
    renderDialog();
    const user = await checkFile();

    const notice = screen.getByTestId("import-match-notice");
    expect(notice).toHaveAttribute("data-variant", "warning");
    expect(notice).toHaveTextContent("An existing account has the same name and number.");
    expect(notice).not.toHaveTextContent("(closed)");

    const link = within(notice).getByTestId("import-match-link");
    expect(link).toHaveAttribute("href", routes.bankAccount("acct-1"));
    expect(link).toHaveTextContent("Import snapshots");

    // Not blocking: still enabled, now the secondary button, labelled for what it does.
    const commit = screen.getByTestId("import-commit-btn");
    expect(commit).toBeEnabled();
    expect(commit).toHaveTextContent("Create anyway");
    expect(commits).toHaveLength(0);

    await user.click(commit);
    await screen.findByTestId("import-done");
    expect(commits).toHaveLength(1);
  });

  it("follows the link to the original's detail page", async () => {
    stubImport({ ...cleanPreview, matches: [existing] });
    renderDialog();
    const user = await checkFile();

    await user.click(screen.getByTestId("import-match-link"));
    await waitFor(() => expect(screen.getByTestId("landed")).toBeInTheDocument());
  });

  it("marks a terminated match as closed", async () => {
    stubImport({ ...cleanPreview, matches: [{ ...existing, status: "closed" }] });
    renderDialog();
    await checkFile();

    expect(screen.getByTestId("import-match-notice")).toHaveTextContent(
      "An existing account (closed) has the same name and number.",
    );
  });

  it("says only 'name' when the subtype has no identifier", async () => {
    stubImport({
      ...cleanPreview,
      name: "Family house",
      identifier: undefined,
      matches: [{ ...existing, group: "asset", subtype: "property" }],
    });
    renderDialog();
    await checkFile();

    const notice = screen.getByTestId("import-match-notice");
    expect(notice).toHaveTextContent("An existing account has the same name.");
    expect(notice).not.toHaveTextContent("and number");
    expect(screen.getByTestId("import-match-link")).toHaveAttribute(
      "href",
      routes.property("acct-1"),
    );
  });

  it("lists every match when there are several", async () => {
    stubImport({
      ...cleanPreview,
      matches: [existing, { ...existing, id: "acct-9", status: "closed" }],
    });
    renderDialog();
    await checkFile();

    const items = screen.getAllByTestId("import-match-item");
    expect(items).toHaveLength(2);
    expect(within(items[1]).getByTestId("import-match-link")).toHaveAttribute(
      "href",
      routes.bankAccount("acct-9"),
    );
  });

  it("shows no notice while the file still has errors", async () => {
    stubImport({
      ...cleanPreview,
      would_create: false,
      field_errors: [{ field: "display_name", message: "is required" }],
      matches: [existing],
    });
    const user = userEvent.setup();
    renderDialog();
    await user.click(screen.getByTestId("import-position-trigger"));
    await user.upload(
      screen.getByTestId("import-file-input"),
      new File([new Uint8Array([1])], "bad.xlsx", { type: XLSX }),
    );
    await user.click(screen.getByTestId("import-check-btn"));
    await screen.findByTestId("import-errors");

    expect(screen.queryByTestId("import-match-notice")).not.toBeInTheDocument();
    expect(screen.getByTestId("import-commit-btn")).toBeDisabled();
  });
});
