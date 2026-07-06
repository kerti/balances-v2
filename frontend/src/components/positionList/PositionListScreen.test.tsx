// The abstraction-contract test for the descriptor-driven list core (ADR-0043).
// It runs against a *synthetic* position type ("widget") — never a real domain
// type — so it asserts the generic's contract, not one type's incidentals. One
// suite here covers list behaviour for all ten real types.
//
// covers: INV-PRESENTATION-04, INV-PRESENTATION-05
import { describe, it, expect, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionListScreen } from "./PositionListScreen";
import type { PositionDeleteMutation, PositionListDescriptor, PositionListQuery } from "./types";

type Widget = {
  id: string;
  label: string;
  sub: string;
  kind: string;
  state: string;
  value: string | null;
};

const widgets: Widget[] = [
  { id: "w1", label: "Alpha", sub: "alpha-sub", kind: "alpha-kind", state: "active", value: "100" }, // prettier-ignore
  { id: "w2", label: "Bravo", sub: "bravo-sub", kind: "bravo-kind", state: "active", value: "300" }, // prettier-ignore
  { id: "w3", label: "Charlie", sub: "charlie-sub", kind: "charlie-kind", state: "closed", value: "50" }, // prettier-ignore
];

function loaded(data: Widget[]): PositionListQuery<Widget> {
  return { data, isPending: false, error: null };
}

function makeDescriptor(
  query: PositionListQuery<Widget>,
  deleteMutate = vi.fn(),
): PositionListDescriptor<Widget> {
  return {
    entityKey: "widget",
    testIdPrefix: "widget",
    group: "assets",
    i18nNamespaces: ["common", "errors"],
    defaultSortKey: "name",
    keys: {
      // Raw strings, not real catalog keys: i18next returns the key unchanged
      // when it can't resolve it, which is all a synthetic type needs.
      listTitle: "Widgets",
      listSubtitle: "All widgets",
      emptyTitle: "No widgets here",
      emptyBody: "Add one to get started",
      noun: "widget",
      nounPlural: "widgets",
      valueLabel: "Value",
      rowActions: "Widget actions",
      deleteTitle: "Delete widget",
    },
    useList: () => query,
    useDelete: () =>
      ({
        mutate: deleteMutate,
        isPending: false,
      }) as unknown as PositionDeleteMutation,
    getId: (w) => w.id,
    getName: (w) => w.label,
    getStatus: (w) => w.state,
    getSnapshot: (w) =>
      w.value
        ? {
            amount: w.value,
            currency: "USD",
            year_month: "2026-01-01T00:00:00Z",
          }
        : null,
    getSecondary: (w) => w.sub,
    deleteDescription: (w) => `Really delete ${w.label}?`,
    extraColumns: [
      {
        id: "kind",
        labelKey: "Kind",
        slot: "main",
        render: (w) => w.kind,
        sort: { key: "kind", type: "text", value: (w) => w.kind },
      },
    ],
    renderHeadline: () => null,
    renderCreateDialog: () => <button type="button">{"Create widget"}</button>,
    renderEditDialog: (w, props) =>
      props.open ? <div role="dialog">{`Editing ${w.label}`}</div> : null,
  };
}

function renderList(query: PositionListQuery<Widget>, deleteMutate = vi.fn()) {
  return renderWithProviders(
    <PositionListScreen descriptor={makeDescriptor(query, deleteMutate)} onSelect={vi.fn()} />,
  );
}

describe("PositionListScreen (synthetic descriptor)", () => {
  it("always renders the shared surface for each active row", () => {
    renderList(loaded(widgets));
    const rows = screen.getAllByTestId("widget-row");
    expect(rows).toHaveLength(2); // Charlie is closed → hidden by default
    // Name + secondary + status + value are the four shared columns.
    const alpha = rows.find((r) => r.textContent?.includes("Alpha"))!;
    expect(within(alpha).getByText("alpha-sub")).toBeInTheDocument();
    expect(within(alpha).getByText("Active")).toBeInTheDocument();
    expect(within(alpha).getByText(/100/)).toBeInTheDocument();
  });

  it("renders group extra columns on the web table", () => {
    renderList(loaded(widgets));
    // The "kind" extra column is present on the dense table.
    expect(screen.getByText("alpha-kind")).toBeInTheDocument();
    expect(screen.getByText("bravo-kind")).toBeInTheDocument();
  });

  it("toggles sort on a column header", async () => {
    const user = userEvent.setup();
    renderList(loaded(widgets));

    // Default: name ascending → Alpha before Bravo.
    let rows = screen.getAllByTestId("widget-row");
    expect(rows[0]).toHaveTextContent("Alpha");

    // Click value → its default direction is descending → 300 (Bravo) first.
    await user.click(screen.getByTestId("sort-value"));
    rows = screen.getAllByTestId("widget-row");
    expect(rows[0]).toHaveTextContent("Bravo");

    // Click the same column again → toggles to ascending → Alpha first.
    await user.click(screen.getByTestId("sort-value"));
    rows = screen.getAllByTestId("widget-row");
    expect(rows[0]).toHaveTextContent("Alpha");
  });

  it("hides inactive rows until the show-inactive toggle is ticked", async () => {
    const user = userEvent.setup();
    renderList(loaded(widgets));
    expect(screen.queryByText("Charlie")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("show-inactive"));
    expect(screen.getByText("Charlie")).toBeInTheDocument();
  });

  it("renders the loading state", () => {
    renderList({ data: undefined, isPending: true, error: null });
    expect(screen.getByText(i18n.t("common:loading"))).toBeInTheDocument();
    expect(screen.queryByTestId("widget-row")).not.toBeInTheDocument();
  });

  it("renders the error state with the failure message", () => {
    renderList({ data: undefined, isPending: false, error: new Error("boom") });
    expect(screen.getByText(/boom/)).toBeInTheDocument();
  });

  it("renders the empty state with a create affordance", () => {
    renderList(loaded([]));
    expect(screen.getByText("No widgets here")).toBeInTheDocument();
    // Two create affordances in the empty state: the header + the empty card.
    expect(screen.getAllByRole("button", { name: "Create widget" })).toHaveLength(2);
  });

  it("opens the edit dialog from the row ⋮ menu", async () => {
    const user = userEvent.setup();
    renderList(loaded(widgets));

    await user.click(screen.getAllByRole("button", { name: "Widget actions" })[0]);
    await user.click(await screen.findByRole("menuitem", { name: "Edit" }));

    expect(await screen.findByText("Editing Alpha")).toBeInTheDocument();
  });

  it("runs the delete flow through the confirm dialog", async () => {
    const user = userEvent.setup();
    const deleteMutate = vi.fn();
    renderList(loaded(widgets), deleteMutate);

    await user.click(screen.getAllByRole("button", { name: "Widget actions" })[0]);
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const confirm = await screen.findByRole("alertdialog");
    expect(within(confirm).getByText("Really delete Alpha?")).toBeInTheDocument();
    await user.click(within(confirm).getByRole("button", { name: "Delete" }));

    expect(deleteMutate).toHaveBeenCalledWith("w1", expect.anything());
  });

  it("drops group extras on the mobile card but keeps the shared surface", () => {
    const original = window.innerWidth;
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 500,
    });
    try {
      renderList(loaded(widgets));
      // Cards, not a table.
      expect(screen.getAllByTestId("widget-card").length).toBe(2);
      expect(screen.queryByTestId("widget-row")).not.toBeInTheDocument();
      // Shared surface stays: name + secondary + value.
      expect(screen.getByText("Alpha")).toBeInTheDocument();
      expect(screen.getByText("alpha-sub")).toBeInTheDocument();
      expect(screen.getByText(/100/)).toBeInTheDocument();
      // The opt-out "kind" extra is hidden on mobile.
      expect(screen.queryByText("alpha-kind")).not.toBeInTheDocument();
    } finally {
      Object.defineProperty(window, "innerWidth", {
        configurable: true,
        value: original,
      });
    }
  });
});
