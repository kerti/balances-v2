// Renderer-pick contract for the HistorySection primitive (ADR-0051 Phase B).
// Below 768px it renders a stacked card list *only when the shape supplies
// `renderCard`*; otherwise — desktop, or a shape not yet carded — it stays on
// the wide table. The primitive stays column-neutral: it forwards to
// `renderRow` / `renderCard` and never inspects the row.
import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { TableCell, TableHead, TableRow } from "@/components/ui/table";
import { HistorySection } from "./HistorySection";
import type { HistorySectionSpec } from "./types";
import i18n from "@/i18n";

vi.mock("@/hooks/use-mobile", () => ({ useIsMobile: vi.fn() }));
import { useIsMobile } from "@/hooks/use-mobile";
const mockUseIsMobile = vi.mocked(useIsMobile);

type Row = { id: string; label: string };

const rows: Row[] = [
  { id: "r1", label: "May 2026" },
  { id: "r2", label: "June 2026" },
];

function baseSpec(overrides: Partial<HistorySectionSpec<Row>> = {}): HistorySectionSpec<Row> {
  return {
    testId: "tour-snapshots",
    title: "History",
    emptyText: "No snapshots yet.",
    header: (
      <TableRow>
        <TableHead>{i18n.t("common:tableHeaders.month")}</TableHead>
      </TableRow>
    ),
    rows,
    renderRow: (row) => (
      <TableRow key={row.id} data-testid={`table-row-${row.id}`}>
        <TableCell>{row.label}</TableCell>
      </TableRow>
    ),
    pageSize: 12,
    ...overrides,
  };
}

const renderCard: HistorySectionSpec<Row>["renderCard"] = (row) => (
  <div key={row.id} data-testid={`card-${row.id}`}>
    {row.label}
  </div>
);

describe("HistorySection renderer pick", () => {
  beforeEach(() => mockUseIsMobile.mockReset());

  it("renders the table on desktop even when a card renderer exists", () => {
    mockUseIsMobile.mockReturnValue(false);
    render(<HistorySection {...baseSpec({ renderCard })} />);
    expect(screen.getByTestId("tour-snapshots-table")).toBeInTheDocument();
    expect(screen.queryByTestId("tour-snapshots-cards")).not.toBeInTheDocument();
    expect(screen.getByTestId("table-row-r1")).toBeInTheDocument();
  });

  it("renders the card list on mobile when the shape supplies renderCard", () => {
    mockUseIsMobile.mockReturnValue(true);
    render(<HistorySection {...baseSpec({ renderCard })} />);
    expect(screen.getByTestId("tour-snapshots-cards")).toBeInTheDocument();
    expect(screen.queryByTestId("tour-snapshots-table")).not.toBeInTheDocument();
    expect(screen.getByTestId("card-r1")).toBeInTheDocument();
    expect(screen.getByTestId("card-r2")).toBeInTheDocument();
  });

  it("falls back to the table on mobile when the shape has no card renderer", () => {
    mockUseIsMobile.mockReturnValue(true);
    render(<HistorySection {...baseSpec()} />);
    expect(screen.getByTestId("tour-snapshots-table")).toBeInTheDocument();
    expect(screen.queryByTestId("tour-snapshots-cards")).not.toBeInTheDocument();
  });

  it("shows the empty copy when there are no rows, at either width", () => {
    mockUseIsMobile.mockReturnValue(true);
    render(<HistorySection {...baseSpec({ rows: [], renderCard })} />);
    expect(screen.getByText("No snapshots yet.")).toBeInTheDocument();
    expect(screen.queryByTestId("tour-snapshots-cards")).not.toBeInTheDocument();
  });
});
