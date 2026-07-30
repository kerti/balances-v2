// The ⋮ row menu must open on a bare `click`, with no `pointerdown` (#572).
//
// Radix's menu trigger opens on `pointerdown` or the Enter/Space/ArrowDown keys
// and has no `click` path at all, which left every card's ⋮ completely dead on
// iOS Safari — the content never mounted. `useMenuOpenOnClick` adds the missing
// path. jsdom can model the failure exactly, because dispatching only `click` is
// precisely what the phone was doing, so this is a real regression test rather
// than a class-contract stand-in: revert the hook and it fails.
//
// covers: INV-PRESENTATION-08
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { RowActionsMenu } from "./RowActionsMenu";

function renderMenu(onEdit = vi.fn(), onDelete = vi.fn()) {
  render(<RowActionsMenu label="Row actions" onEdit={onEdit} onDelete={onDelete} />);
  return screen.getByRole("button", { name: "Row actions" });
}

describe("RowActionsMenu", () => {
  it("opens from a click alone, without pointerdown", () => {
    const trigger = renderMenu();
    fireEvent.click(trigger);

    expect(screen.getByRole("menu")).toBeInTheDocument();
  });

  it("still reaches both actions from a click-opened menu", () => {
    const onEdit = vi.fn();
    const onDelete = vi.fn();
    const trigger = renderMenu(onEdit, onDelete);

    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("menuitem", { name: "Edit" }));
    expect(onEdit).toHaveBeenCalledOnce();

    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("menuitem", { name: "Delete" }));
    expect(onDelete).toHaveBeenCalledOnce();
  });

  // The fallback must not double-fire where pointerdown already works: Radix
  // opens on pointerdown, and the click that follows has to be a no-op rather
  // than a second toggle that closes the menu again.
  it("does not toggle back shut when pointerdown opened it first", () => {
    const trigger = renderMenu();

    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);

    expect(screen.getByRole("menu")).toBeInTheDocument();
  });

  // The trigger sits inside a card that navigates on click.
  it("keeps the click off the card underneath", () => {
    const onSelect = vi.fn();
    render(
      <div onClick={onSelect}>
        <RowActionsMenu label="Row actions" onEdit={vi.fn()} onDelete={vi.fn()} />
      </div>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Row actions" }));
    expect(onSelect).not.toHaveBeenCalled();
  });
});
