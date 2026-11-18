import { describe, expect, it } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { HintedLabel, InfoHint } from "@/components/common/InfoHint";

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

describe("HintedLabel", () => {
  // covers: INV-PRESENTATION-31
  it("glues the hint to the label's last word so it can't wrap alone (#931)", () => {
    renderWithProviders(
      <HintedLabel label="Perubahan nilai properti & kendaraan" hint="Non-tunai." testId="h" />,
    );
    const glue = screen.getByTestId("h").parentElement!;
    expect(glue).toHaveClass("whitespace-nowrap");
    expect(glue).toHaveTextContent(/^kendaraan$/);
    expect(screen.getByRole("button", { name: "Perubahan nilai properti & kendaraan" })).toBe(
      screen.getByTestId("h"),
    );
  });

  it("renders a plain label when there is no hint", () => {
    const { container } = renderWithProviders(<HintedLabel label="Pemasukan" />);
    expect(container).toHaveTextContent("Pemasukan");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
