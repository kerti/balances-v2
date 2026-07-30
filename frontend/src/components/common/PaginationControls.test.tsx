// Renderer conformance for the pagination window (#572). The control used to
// render one link per page unconditionally, so a long history pushed the last
// pages and the Next arrow off the right edge of a phone. Flips
// window.innerWidth across the 768px boundary so useIsMobile() picks the sibling
// count, then asserts the divergence contract: fewer items on a phone, first and
// last always reachable in both, and the items at the ≥44px tap floor below
// 768px — `size="icon"` is the one Button family the #559 floor skips, so
// pagination sat at 32px until this.
//
// jsdom has no layout, so the floor assertion is on the class contract (same
// rationale as `tapFloor.test.tsx`); the paging behaviour itself is real.
//
// covers: INV-PRESENTATION-08
import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PaginationControls } from "./PaginationControls";

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

function pageLabels() {
  return screen.getAllByTestId("pagination-page").map((el) => el.textContent);
}

describe("PaginationControls", () => {
  it("truncates a long range instead of running off the phone", () => {
    setViewport(390);
    render(<PaginationControls page={20} totalPages={40} onPageChange={vi.fn()} />);

    expect(pageLabels()).toEqual(["1", "20", "40"]);
    expect(screen.getAllByTestId("pagination-ellipsis")).toHaveLength(2);
  });

  it("shows the flanking pages once there is room for them", () => {
    setViewport(1280);
    render(<PaginationControls page={20} totalPages={40} onPageChange={vi.fn()} />);

    expect(pageLabels()).toEqual(["1", "19", "20", "21", "40"]);
  });

  it("keeps every page when they all fit", () => {
    setViewport(390);
    render(<PaginationControls page={2} totalPages={4} onPageChange={vi.fn()} />);

    expect(pageLabels()).toEqual(["1", "2", "3", "4"]);
    expect(screen.queryByTestId("pagination-ellipsis")).not.toBeInTheDocument();
  });

  it("floors the page targets on phones", () => {
    setViewport(390);
    render(<PaginationControls page={20} totalPages={40} onPageChange={vi.fn()} />);

    for (const link of screen.getAllByTestId("pagination-page")) {
      expect(link).toHaveClass("max-md:size-11");
    }
    // Below `sm` the prev/next labels are hidden, so those two need a width
    // floor as well as Button's height floor.
    expect(screen.getByTestId("pagination-prev")).toHaveClass("max-sm:w-11");
    expect(screen.getByTestId("pagination-next")).toHaveClass("max-sm:w-11");
  });

  it("still pages from a truncated control", async () => {
    setViewport(390);
    const onPageChange = vi.fn();
    render(<PaginationControls page={20} totalPages={40} onPageChange={onPageChange} />);

    await userEvent.click(screen.getByTestId("pagination-next"));
    expect(onPageChange).toHaveBeenLastCalledWith(21);

    await userEvent.click(screen.getByTestId("pagination-prev"));
    expect(onPageChange).toHaveBeenLastCalledWith(19);

    // The jump the truncated control still has to offer.
    await userEvent.click(screen.getByRole("link", { name: "40" }));
    expect(onPageChange).toHaveBeenLastCalledWith(40);
  });

  it("does not page past either end", async () => {
    setViewport(1280);
    const onPageChange = vi.fn();
    const { unmount } = render(
      <PaginationControls page={1} totalPages={40} onPageChange={onPageChange} />,
    );
    await userEvent.click(screen.getByTestId("pagination-prev"));
    expect(onPageChange).not.toHaveBeenCalled();
    unmount();

    render(<PaginationControls page={40} totalPages={40} onPageChange={onPageChange} />);
    await userEvent.click(screen.getByTestId("pagination-next"));
    expect(onPageChange).not.toHaveBeenCalled();
  });
});
