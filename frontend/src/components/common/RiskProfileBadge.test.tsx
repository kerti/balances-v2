// covers: INV-PRESENTATION-23
import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { RiskProfileBadge } from "@/components/common/RiskProfileBadge";
import type { RiskProfile } from "@/api/types";

const cases: [RiskProfile, string][] = [
  ["low", "L"],
  ["medium", "M"],
  ["high", "H"],
];

describe("RiskProfileBadge", () => {
  it.each(cases)(
    "%s is monochrome but keeps its letter and accessible label",
    (profile, letter) => {
      const { container } = renderWithProviders(<RiskProfileBadge profile={profile} />);
      expect(container.innerHTML).not.toMatch(/emerald|amber|rose|green|red-/);
      expect(screen.getByTestId(`risk-profile-${profile}`).className).toContain(
        "text-muted-foreground",
      );
      expect(screen.getByText(letter)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /\w/ })).toBeInTheDocument();
    },
  );
});
