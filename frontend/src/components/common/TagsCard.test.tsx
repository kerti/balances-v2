// Tag delete is the one Undo-tier exception (ADR-0058 §3, amended 2026-11-10):
// deleting a tag clears every item's assignment irrecoverably and no undo
// endpoint exists, so the card asks first, in red, and then toasts plainly.
import { it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, fireEvent, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
import { renderWithProviders } from "@/test/renderWithProviders";
import { TagsCard } from "@/components/common/TagsCard";
import type { Tag } from "@/api/types";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const tags: Tag[] = [
  {
    id: "t1",
    household_id: "hh-1",
    name: "Retirement",
    color: "#112233",
    created_by: null,
    created_at: "2026-05-01T00:00:00Z",
    updated_by: null,
    updated_at: "2026-05-01T00:00:00Z",
    deleted_at: null,
  },
];

// covers: INV-PRESENTATION-16
it("asks first in red with the count-free copy, then toasts Deleted with no Undo", async () => {
  let deletes = 0;
  server.use(
    http.get("/api/tags", () => HttpResponse.json(tags)),
    http.delete("/api/tags/:id", () => {
      deletes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderWithProviders(<TagsCard />);

  fireEvent.click(await screen.findByRole("button", { name: "Tag actions" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Delete" }));
  const dialog = await screen.findByRole("alertdialog");
  expect(within(dialog).getByText("Delete tag Retirement?")).toBeInTheDocument();
  expect(within(dialog).getByText("It's removed from every item that has it.")).toBeInTheDocument();
  expect(deletes).toBe(0);

  const confirm = within(dialog).getByRole("button", { name: "Delete" });
  expect(confirm).toHaveAttribute("data-variant", "destructive");
  fireEvent.click(confirm);

  await waitFor(() => expect(deletes).toBe(1));
  await waitFor(() => expect(raisedToasts(mockedToast)).toHaveLength(1));
  const [only] = raisedToasts(mockedToast);
  expect(only.message).toBe("Tag deleted");
  expect(only.options.action).toBeUndefined();
});

it("New tag opens a dialog; Edit renames in a dialog with no inline form", async () => {
  let created: unknown = null;
  let patched: unknown = null;
  server.use(
    http.get("/api/tags", () => HttpResponse.json(tags)),
    http.post("/api/tags", async ({ request }) => {
      created = await request.json();
      return HttpResponse.json(tags[0]);
    }),
    http.patch("/api/tags/:id", async ({ request }) => {
      patched = await request.json();
      return HttpResponse.json(tags[0]);
    }),
  );
  renderWithProviders(<TagsCard />);

  // No inline name field or ✓/× controls: the name only exists inside a dialog.
  await screen.findByTestId("tag-list");
  expect(screen.queryByTestId("new-tag-name")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "New tag" }));
  let dialog = within(await screen.findByRole("dialog"));
  const add = dialog.getByRole("button", { name: "Add" });
  expect(add).toBeEnabled();
  fireEvent.click(add);
  expect(dialog.getByTestId("field-error-new-tag-name")).toBeInTheDocument();
  fireEvent.change(dialog.getByLabelText("Name"), { target: { value: "  Travel " } });
  fireEvent.click(add);
  await waitFor(() => expect(created).toMatchObject({ name: "Travel" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

  fireEvent.click(await screen.findByRole("button", { name: "Tag actions" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Edit" }));
  dialog = within(await screen.findByRole("dialog"));
  expect(dialog.getByLabelText("Name")).toHaveValue("Retirement");
  fireEvent.change(dialog.getByLabelText("Name"), { target: { value: "Pension" } });
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));
  await waitFor(() => expect(patched).toMatchObject({ name: "Pension" }));
});

it("a tag's row menu offers Add items…, opening the picker preset to that tag", async () => {
  let put: unknown = null;
  server.use(
    http.get("/api/tags", () => HttpResponse.json(tags)),
    http.get("/api/tags/items", () =>
      HttpResponse.json([
        {
          group: "receivable",
          id: "r1",
          subtype: "receivable",
          name: "Owed Loan",
          tag_id: null,
          currency: null,
          amount: null,
          terminated: false,
        },
      ]),
    ),
    http.put("/api/tags/assignments", async ({ request }) => {
      put = await request.json();
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderWithProviders(<TagsCard />);

  const trigger = await screen.findByRole("button", { name: "Tag actions" });
  // Phone tap floor: the ⋯ trigger is 44px below the md breakpoint.
  expect(trigger).toHaveClass("max-md:size-11");
  fireEvent.click(trigger);
  fireEvent.click(screen.getByRole("menuitem", { name: "Add items…" }));

  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getByText("Add items to Retirement")).toBeInTheDocument();
  // Preset: no tag chooser.
  expect(within(dialog).queryByTestId("add-items-target")).not.toBeInTheDocument();
  fireEvent.click(await within(dialog).findByTestId("add-items-option-r1"));
  fireEvent.click(within(dialog).getByTestId("add-items-save"));
  await waitFor(() =>
    expect(put).toEqual({ tag_id: "t1", items: [{ group: "receivable", position_id: "r1" }] }),
  );
});
