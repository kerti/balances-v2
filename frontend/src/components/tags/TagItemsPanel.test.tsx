// The Tags page's per-tag item lists (#853): expand a row to see exactly the
// items carrying the tag, remove one (Undo toast), and add several in one save
// (one toast, the breakdown refetched). Rendered through TagsScreen so the
// query wiring and invalidation are the real ones.
import { describe, expect, it, vi, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
import { renderWithProviders } from "@/test/renderWithProviders";
import { TagsScreen } from "@/components/screens/TagsScreen";
import type { Tag, TaggableItem, TagBreakdownRow } from "@/api/types";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

function tag(id: string, name: string): Tag {
  return {
    id,
    household_id: "hh",
    name,
    color: "#2563eb",
    created_by: null,
    created_at: "2026-05-10T00:00:00Z",
    updated_by: null,
    updated_at: "2026-05-10T00:00:00Z",
    deleted_at: null,
  };
}

function item(over: Partial<TaggableItem> & Pick<TaggableItem, "id" | "name">): TaggableItem {
  return {
    group: "investment",
    subtype: "stock",
    tag_id: null,
    currency: "IDR",
    amount: "1000",
    terminated: false,
    ...over,
  };
}

const tags = [tag("t-ret", "Retirement"), tag("t-trv", "Travel")];
const rows: TagBreakdownRow[] = [
  { tag_id: "t-ret", group: "investment", currency: "IDR", total: "5000" },
  { tag_id: "t-trv", group: "asset", currency: "IDR", total: "2000" },
  { tag_id: null, group: "receivable", currency: "IDR", total: "300" },
];

function seed() {
  return [
    item({ id: "s1", name: "Alpha Shares", tag_id: "t-ret", amount: "3000" }),
    item({ id: "s2", name: "Beta Shares", tag_id: "t-ret", amount: "2000" }),
    item({ id: "b1", name: "Cash Pot", group: "asset", subtype: "bank_account", tag_id: "t-trv" }),
    item({ id: "b2", name: "Day Fund", group: "asset", subtype: "bank_account", tag_id: "t-trv" }),
    item({
      id: "u1",
      name: "Owed Loan",
      group: "receivable",
      subtype: "receivable",
      amount: "300",
    }),
    item({ id: "u2", name: "Old Bond", subtype: "bond", terminated: true }),
    item({
      id: "u3",
      name: "Unvalued Plot",
      group: "asset",
      subtype: "property",
      currency: null,
      amount: null,
    }),
    item({ id: "u4", name: "Extra Gold", subtype: "gold" }),
    item({ id: "u5", name: "Spare Stock", subtype: "stock" }),
  ];
}

type Put = { tag_id: string | null; items: { group: string; position_id: string }[] };

function setup() {
  let items = seed();
  const puts: Put[] = [];
  let breakdownFetches = 0;
  server.use(
    http.get("/api/tags", () => HttpResponse.json(tags)),
    http.get("/api/tags/breakdown", () => {
      breakdownFetches++;
      return HttpResponse.json(rows);
    }),
    http.get("/api/tags/items", () => HttpResponse.json(items)),
    http.put("/api/tags/assignments/bulk", async ({ request }) => {
      const body = (await request.json()) as Put;
      puts.push(body);
      const ids = new Set(body.items.map((i) => i.position_id));
      items = items.map((it) => (ids.has(it.id) ? { ...it, tag_id: body.tag_id } : it));
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderWithProviders(
    <MemoryRouter>
      <TagsScreen />
    </MemoryRouter>,
  );
  return { puts, fetches: () => breakdownFetches };
}

const originalWidth = window.innerWidth;
afterEach(() => {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: originalWidth });
  vi.clearAllMocks();
});

describe("Tags page item lists", () => {
  it("expanding a tag lists exactly its items", async () => {
    setup();
    await userEvent.click(await screen.findByTestId("tag-expand-t-ret-IDR"));

    const panel = await screen.findByTestId("tag-items-t-ret-IDR");
    const names = within(panel)
      .getAllByTestId("tag-item-link")
      .map((a) => a.textContent);
    expect(names).toEqual(["Alpha Shares", "Beta Shares"]);
    expect(within(panel).getAllByTestId("tag-item-value")[0]).toHaveTextContent(/3,000/);
    expect(within(panel).getAllByTestId("tag-item-link")[0]).toHaveAttribute(
      "href",
      "/investments/stocks/s1",
    );
    expect(within(panel).getAllByText("Stock").length).toBe(2);
  });

  it("remove x untags one item and offers Undo", async () => {
    const { puts } = setup();
    await userEvent.click(await screen.findByTestId("tag-expand-t-ret-IDR"));
    const panel = await screen.findByTestId("tag-items-t-ret-IDR");
    await userEvent.click(within(panel).getAllByTestId("tag-item-remove")[0]);

    await waitFor(() =>
      expect(puts).toEqual([{ tag_id: null, items: [{ group: "investment", position_id: "s1" }] }]),
    );
    // One toast, carrying Undo (ADR-0058 tier: single-item write).
    await waitFor(() => expect(raisedToasts(toast as never)).toHaveLength(1));
    const [t] = raisedToasts(toast as never);
    expect(t.message).toBe("Removed Alpha Shares from Retirement");
    expect(t.options.action?.label).toBe("Undo");

    // The list refetches without the removed item.
    await waitFor(() =>
      expect(
        within(panel)
          .getAllByTestId("tag-item-link")
          .map((a) => a.textContent),
      ).toEqual(["Beta Shares"]),
    );

    t.options.action?.onClick();
    await waitFor(() =>
      expect(puts[1]).toEqual({
        tag_id: "t-ret",
        items: [{ group: "investment", position_id: "s1" }],
      }),
    );
  });

  // covers: INV-TAGS-06
  it("adding 5 items (2 from another tag) moves them in one save and refreshes the breakdown", async () => {
    const { puts, fetches } = setup();
    await userEvent.click(await screen.findByTestId("tag-expand-t-ret-IDR"));
    await userEvent.click(await screen.findByTestId("tag-items-add"));

    const dialog = await screen.findByRole("dialog");
    // Active items only: the terminated bond is not offered, nor are items the
    // tag already carries.
    expect(within(dialog).queryByText("Old Bond")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("Alpha Shares")).not.toBeInTheDocument();
    // An item with another tag says so.
    expect(within(dialog).getAllByText("currently: Travel")).toHaveLength(2);

    // Search narrows, and the picks survive it.
    await userEvent.type(within(dialog).getByTestId("add-items-search"), "day");
    expect(within(dialog).queryByText("Cash Pot")).not.toBeInTheDocument();
    await userEvent.click(within(dialog).getByTestId("add-items-option-b2"));
    await userEvent.clear(within(dialog).getByTestId("add-items-search"));
    for (const id of ["b1", "u1", "u3", "u4"]) {
      await userEvent.click(within(dialog).getByTestId(`add-items-option-${id}`));
    }

    const before = fetches();
    await userEvent.click(within(dialog).getByTestId("add-items-save"));

    await waitFor(() => expect(puts).toHaveLength(1));
    expect(puts[0].tag_id).toBe("t-ret");
    expect(puts[0].items.map((i) => i.position_id).sort()).toEqual(["b1", "b2", "u1", "u3", "u4"]);
    await waitFor(() => expect(raisedToasts(toast as never)).toHaveLength(1));
    expect(raisedToasts(toast as never)[0].message).toBe(
      "Added 5 items to Retirement · 2 moved from another tag",
    );
    await waitFor(() => expect(fetches()).toBeGreaterThan(before));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("saving with nothing selected shows an inline error and writes nothing", async () => {
    const { puts } = setup();
    await userEvent.click(await screen.findByTestId("tag-expand-t-ret-IDR"));
    await userEvent.click(await screen.findByTestId("tag-items-add"));
    await userEvent.click(await screen.findByTestId("add-items-save"));
    expect(await screen.findByTestId("field-error-pick_items")).toHaveTextContent(
      "Select at least one item.",
    );
    expect(puts).toHaveLength(0);
  });

  it("Untagged lists untagged items and Add to a tag asks for the tag", async () => {
    const { puts } = setup();
    await userEvent.click(await screen.findByTestId("tag-expand-untagged-IDR"));
    const panel = await screen.findByTestId("tag-items-untagged-IDR");
    expect(
      within(panel)
        .getAllByTestId("tag-item-link")
        .map((a) => a.textContent),
    ).toEqual(["Owed Loan", "Old Bond", "Extra Gold", "Spare Stock"]);
    // Untagged items carry no tag, so there is no remove x.
    expect(within(panel).queryByTestId("tag-item-remove")).not.toBeInTheDocument();

    await userEvent.click(within(panel).getByTestId("tag-items-add-untagged"));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByTestId("add-items-option-u5"));
    await userEvent.click(within(dialog).getByTestId("add-items-save"));
    expect(await within(dialog).findByTestId("field-error-target_tag")).toBeInTheDocument();
    expect(puts).toHaveLength(0);

    await userEvent.selectOptions(within(dialog).getByTestId("add-items-target"), "t-trv");
    await userEvent.click(within(dialog).getByTestId("add-items-save"));
    await waitFor(() =>
      expect(puts).toEqual([
        { tag_id: "t-trv", items: [{ group: "investment", position_id: "u5" }] },
      ]),
    );
  });

  it("mobile: expand toggle, links and remove clear the 44px tap floor", async () => {
    Object.defineProperty(window, "innerWidth", { configurable: true, value: 500 });
    setup();
    const toggle = await screen.findByTestId("tag-expand-t-ret-IDR");
    expect(toggle).toHaveClass("min-h-11");
    await userEvent.click(toggle);
    const panel = await screen.findByTestId("tag-items-t-ret-IDR");
    expect(within(panel).getAllByTestId("tag-item-remove")[0]).toHaveClass("size-11");
    expect(within(panel).getAllByTestId("tag-item-link")[0]).toHaveClass("min-h-11");
  });
});
