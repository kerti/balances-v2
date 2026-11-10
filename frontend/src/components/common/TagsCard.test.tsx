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

  fireEvent.click(await screen.findByTestId("delete-tag"));
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
