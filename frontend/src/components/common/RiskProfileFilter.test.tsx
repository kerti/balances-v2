// Renderer conformance for the investment-subtype list risk-profile filter
// (#542). The chip bar is a diverged surface control: on phones it fills the
// width and its options split it evenly, each clearing the ≥44px tap floor;
// from 768px up it collapses to natural, content-width chips. Layout is pure
// CSS (`max-md:` utilities), so this asserts the class contract rather than
// measuring pixels — the mobile a11y floor of ADR-0050.
//
// covers: INV-PRESENTATION-08
import { it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { RiskProfileFilter } from "@/components/common/RiskProfileFilter";

it("fills the width and floors each option at the 44px tap target on phones", () => {
  renderWithProviders(<RiskProfileFilter value="all" onChange={vi.fn()} />);

  // The bar stretches full width and distributes its options evenly on phones.
  const group = screen.getByRole("group");
  expect(group).toHaveClass("max-md:w-full", "max-md:[&>*]:flex-1");

  // Every option (All / Low / Medium / High) clears the tap floor on phones and
  // relaxes back to its natural size from 768px up.
  for (const value of ["all", "low", "medium", "high"]) {
    const chip = screen.getByTestId(`risk-filter-${value}`);
    expect(chip).toHaveClass("max-md:min-h-11");
  }
});
