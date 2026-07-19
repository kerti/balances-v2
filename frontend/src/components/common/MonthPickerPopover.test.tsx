// covers: prev/next arrows on MonthPickerPopover jump to the nearest
// available month (index-neighbour in the sparse `months` list), not the
// nearest calendar month — a multi-year data gap collapses to one click.
import { describe, it, expect, vi } from "vitest";
import { screen, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";

describe("MonthPickerPopover prev/next", () => {
  it("jumps over a multi-year gap in one click", () => {
    const months = ["2011-06-01T00:00:00Z", "2024-01-01T00:00:00Z", "2024-02-01T00:00:00Z"];
    const onSelect = vi.fn();
    renderWithProviders(
      <MonthPickerPopover months={months} selected="2024-01-01T00:00:00Z" onSelect={onSelect} />,
    );

    fireEvent.click(screen.getByTestId("month-picker-prev"));
    expect(onSelect).toHaveBeenCalledWith("2011-06-01T00:00:00Z");
  });

  it("disables prev at the earliest available month", () => {
    const months = ["2011-06-01T00:00:00Z", "2024-01-01T00:00:00Z"];
    renderWithProviders(
      <MonthPickerPopover months={months} selected="2011-06-01T00:00:00Z" onSelect={vi.fn()} />,
    );

    expect(screen.getByTestId("month-picker-prev")).toBeDisabled();
  });

  it("disables next at the latest available month", () => {
    const months = ["2011-06-01T00:00:00Z", "2024-01-01T00:00:00Z"];
    renderWithProviders(
      <MonthPickerPopover months={months} selected="2024-01-01T00:00:00Z" onSelect={vi.fn()} />,
    );

    expect(screen.getByTestId("month-picker-next")).toBeDisabled();
  });

  it("advances to the very next available month when adjacent", () => {
    const months = ["2024-01-01T00:00:00Z", "2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z"];
    const onSelect = vi.fn();
    renderWithProviders(
      <MonthPickerPopover months={months} selected="2024-02-01T00:00:00Z" onSelect={onSelect} />,
    );

    fireEvent.click(screen.getByTestId("month-picker-next"));
    expect(onSelect).toHaveBeenCalledWith("2024-03-01T00:00:00Z");
  });
});
