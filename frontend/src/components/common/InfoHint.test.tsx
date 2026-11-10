import { describe, expect, it } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { InfoHint } from "@/components/common/InfoHint";

describe("InfoHint", () => {
  it("opens its explanation on click", () => {
    renderWithProviders(<InfoHint label="About this row">Because reasons.</InfoHint>);
    expect(screen.queryByText("Because reasons.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "About this row" }));
    expect(screen.getByText("Because reasons.")).toBeInTheDocument();
  });

  it("opens on keyboard focus", () => {
    renderWithProviders(<InfoHint label="About this row">Because reasons.</InfoHint>);
    fireEvent.focus(screen.getByRole("button", { name: "About this row" }));
    expect(screen.getByText("Because reasons.")).toBeInTheDocument();
  });

  it("stays open when focus is followed by the tap's click", () => {
    renderWithProviders(<InfoHint label="About this row">Because reasons.</InfoHint>);
    const button = screen.getByRole("button", { name: "About this row" });
    fireEvent.focus(button);
    fireEvent.click(button);
    expect(screen.getByText("Because reasons.")).toBeInTheDocument();
  });
});
